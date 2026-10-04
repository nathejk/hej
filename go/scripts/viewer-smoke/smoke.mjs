// Viewer video smoke test (PRD 029, tasks 499/500/504): drives the real viewer.js in headless Chrome over CDP.
//
//   node smoke.mjs <video.mp4> <poster.jpg>
//
// Serves a throwaway page with a photograph, the clip as a short (looping) video and the clip again as a "long"
// video with an SD rendition, from a server that honours Range. Needs Google Chrome installed; no npm packages.
import { spawn } from 'node:child_process';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { fileURLToPath } from 'node:url';

const [clip, poster] = process.argv.slice(2);
if (!clip || !poster) { console.error('usage: node smoke.mjs <video.mp4> <poster.jpg>'); process.exit(2); }
const here = path.dirname(fileURLToPath(import.meta.url));
const viewerDir = path.join(here, '../../cmd/api/viewer');
const files = {
  '/viewer.js': [path.join(viewerDir, 'viewer.js'), 'text/javascript'],
  '/viewer.css': [path.join(viewerDir, 'viewer.css'), 'text/css'],
  '/clip.mp4': [clip, 'video/mp4'],
  '/poster.jpg': [poster, 'image/jpeg'],
};
const page = `<!doctype html><html><head><link rel="stylesheet" href="viewer.css"></head><body>
<div id="c" data-viewer data-viewer-actions="download">
 <a id="t0" href="poster.jpg" data-viewer-item data-viewer-ordinal="0" data-full="poster.jpg" data-thumb="poster.jpg">photo</a>
 <a id="t1" href="clip.mp4" data-viewer-item data-viewer-ordinal="1" data-full="clip.mp4?v=1" data-thumb="poster.jpg" data-medium="poster.jpg" data-kind="video" data-duration-ms="8000">short</a>
 <a id="t2" href="clip.mp4" data-viewer-item data-viewer-ordinal="2" data-full="clip.mp4?v=2" data-thumb="poster.jpg" data-kind="video" data-duration-ms="600000" data-sd="clip.mp4?v=2sd">long</a>
</div><script src="viewer.js"></script></body></html>`;
const requests = [];
const server = http.createServer((req, res) => {
  const p = new URL(req.url, 'http://x').pathname;
  if (p === '/' || p === '/index.html') { res.writeHead(200, { 'Content-Type': 'text/html' }); return res.end(page); }
  const f = files[p];
  if (!f) { res.writeHead(404); return res.end(); }
  requests.push(req.url);
  const size = fs.statSync(f[0]).size;
  const m = /bytes=(\d*)-(\d*)/.exec(req.headers.range || '');
  if (m) {
    const start = m[1] ? +m[1] : 0, end = m[2] ? +m[2] : size - 1;
    res.writeHead(206, { 'Content-Type': f[1], 'Accept-Ranges': 'bytes', 'Content-Range': `bytes ${start}-${end}/${size}`, 'Content-Length': end - start + 1 });
    return fs.createReadStream(f[0], { start, end }).pipe(res);
  }
  res.writeHead(200, { 'Content-Type': f[1], 'Accept-Ranges': 'bytes', 'Content-Length': size });
  fs.createReadStream(f[0]).pipe(res);
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));
const base = `http://127.0.0.1:${server.address().port}`;

const port = 9300 + Math.floor(Math.random() * 600);
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'viewer-smoke-'));
const chrome = spawn('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
  ['--headless=new', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, '--no-first-run', '--mute-audio', 'about:blank'], { stdio: 'ignore' });
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
let targets;
for (let i = 0; i < 150; i++) { try { targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json(); break; } catch { await sleep(200); } }
const ws = new WebSocket(targets.find((t) => t.type === 'page').webSocketDebuggerUrl);
await new Promise((r) => ws.addEventListener('open', r));
let id = 0; const pending = new Map();
ws.addEventListener('message', (m) => { const msg = JSON.parse(m.data); if (msg.id && pending.has(msg.id)) { pending.get(msg.id)(msg); pending.delete(msg.id); } });
const send = (method, params = {}) => new Promise((r) => { const i = ++id; pending.set(i, r); ws.send(JSON.stringify({ id: i, method, params })); });
const ev = async (expr) => (await send('Runtime.evaluate', { expression: expr, returnByValue: true, awaitPromise: true })).result.result.value;
const key = async (k, code, mods = 0, text) => {
  await send('Input.dispatchKeyEvent', { type: 'keyDown', key: k, code, modifiers: mods, text });
  await send('Input.dispatchKeyEvent', { type: 'keyUp', key: k, code, modifiers: mods });
};
const click = async (sel) => {
  const r = await ev(`(()=>{const b=document.querySelector('${sel}').getBoundingClientRect();return [b.x+b.width/2,b.y+b.height/2]})()`);
  for (const type of ['mousePressed', 'mouseReleased']) await send('Input.dispatchMouseEvent', { type, x: r[0], y: r[1], button: 'left', clickCount: 1 });
};
const ready = async () => { for (let i = 0; i < 100; i++) { if (await ev('!!(window.hejViewer && document.querySelector("#t2"))')) return; await sleep(200); } };
const closeViewer = () => ev(`document.querySelector('dialog.hv').close()`);
const st = () => ev(`(()=>{const v=document.querySelector('.hv-video');const d=document.querySelector('dialog.hv');const q=document.querySelector('.hv-quality');
  return {open:d&&d.open,isVideo:d&&d.classList.contains('is-video'),missing:d&&d.classList.contains('is-missing'),src:v&&v.getAttribute('src'),muted:v&&v.muted,paused:v&&v.paused,loop:v&&v.loop,t:v&&v.currentTime,
  w:v&&v.videoWidth,h:v&&v.videoHeight,dur:v&&v.duration,unmuteHidden:document.querySelector('.hv-unmute')&&document.querySelector('.hv-unmute').hidden,
  q:q&&q.textContent,qHidden:q&&q.hidden,img:document.querySelector('.hv-img')&&document.querySelector('.hv-img').getAttribute('src')}})()`);
const results = []; const check = (n, ok, x = '') => results.push((ok ? 'PASS ' : 'FAIL ') + n + (x ? ' — ' + x : ''));

await send('Network.enable'); await send('Page.enable');
await send('Page.navigate', { url: base + '/' }); await sleep(500); await ready();

await click('#t1'); await sleep(2000);
let s = await st();
check('click opens and plays the clip', s.open && s.isVideo && !s.missing && s.paused === false && s.t > 0.3, JSON.stringify(s));
check('the clip decodes (has dimensions and duration)', s.w > 0 && s.h > 0 && s.dur > 0, `${s.w}x${s.h}, ${s.dur}s`);
check('opened by click → with sound', s.muted === false);
check('≤15 s loops', s.loop === true);
await key(' ', 'Space', 0, ' '); await sleep(200);
s = await st(); check('Space pauses without closing', s.open && s.paused === true);
const t0 = s.t;
await key('l', 'KeyL', 0, 'l'); await sleep(400);
s = await st(); check('L seeks forward 10 s', s.t > t0 + 9, `${t0} → ${s.t}`);
await key('ArrowLeft', 'ArrowLeft', 8); await sleep(400);
s = await st(); check('Shift+← seeks back 10 s', s.t < t0 + 1.5 && s.src === 'clip.mp4?v=1', `→ ${s.t}`);
await key(' ', 'Space', 0, ' '); await sleep(200);
await key('ArrowRight', 'ArrowRight'); await sleep(2000);
s = await st();
check('→ moves to the next video, muted, with the unmute button', s.src === 'clip.mp4?v=2' && s.muted && !s.unmuteHidden, JSON.stringify(s));
check('long clip does not loop, shows HD toggle', !s.loop && s.q === 'HD' && !s.qHidden);
const at = s.t;
await click('.hv-quality'); await sleep(1500);
s = await st(); check('HD → SD keeps the position and keeps playing', s.src === 'clip.mp4?v=2sd' && s.t >= at - 0.1 && !s.paused, `${at} → ${s.t}`);
await click('.hv-unmute'); await sleep(200);
s = await st(); check('unmute', s.muted === false && s.unmuteHidden);
await key('ArrowLeft', 'ArrowLeft'); await sleep(1500);
s = await st(); check('the unmute choice holds on the next video', s.src === 'clip.mp4?v=1' && s.muted === false, JSON.stringify(s));
await key('ArrowLeft', 'ArrowLeft'); await sleep(800);
s = await st(); check('a photograph releases the video', !s.isVideo && !s.src && s.img === 'poster.jpg');
await key('ArrowRight', 'ArrowRight'); await sleep(800);
await closeViewer(); await sleep(300);
s = await st(); check('closing releases the video', !s.open && !s.src);

await send('Page.reload'); await sleep(500); await ready();
await send('Network.emulateNetworkConditions', { offline: false, latency: 300, downloadThroughput: 50000, uploadThroughput: 20000, connectionType: 'cellular3g' });
await sleep(300);
await click('#t2'); await sleep(1500);
s = await st(); check('a 3g connection starts on SD', s.src === 'clip.mp4?v=2sd', JSON.stringify(s));
await closeViewer();

check('no SD requested before it was chosen, and no neighbour MP4 prefetched',
  requests.filter((u) => u.includes('.mp4')).every((u) => /v=(1|2|2sd)$/.test(u)), requests.filter((u) => u.includes('.mp4')).join(' '));

console.log(results.join('\n'));
ws.close(); chrome.kill(); server.close(); fs.rmSync(profile, { recursive: true, force: true });
process.exit(results.some((r) => r.startsWith('FAIL')) ? 1 : 0);

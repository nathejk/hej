# 108 — Device testing pass: profile page on real phones

**Status:** open
**Priority:** medium
**Created:** 2026-08-28

## Prerequisites (checked 2026-08-28 — read before booking a phone)

1. **The dev stack is not reachable from a phone as it stands.**
   `*.local.nathejk.dk` resolves to **127.0.0.1**, so a phone resolves the app to its
   own loopback. The certificate is a real Let's Encrypt one for
   `hej.local.nathejk.dk` (valid to 2026-10-30), which is the useful part: the name is
   publicly trusted, so it only needs to *resolve* to the dev host
   (`10.10.3.115` at time of writing) for a secure context to work. Options, best
   first:
   - a DNS override on the LAN router (or the phone's DNS) for that name — keeps the
     valid cert and full LAN speed;
   - a tunnel (cloudflared/tailscale) — works off-LAN too;
   - a staging deploy.

   **`https://10.10.3.115` will not work**: Traefik routes on `Host`, so it answers
   404, and the certificate would not match either.

2. **Push cannot be tested on the dev stack.** vite-plugin-pwa registers no service
   worker in dev — `/sw.js` returns `index.html` with `text/html`, and the dev HTML
   has no `registerSW`. iOS Web Push additionally requires the app to be added to the
   Home Screen. So the notification row's "off → enable → subscribed" path and the
   recovery-from-denied path need a **production build** (or staging), not the dev
   stack. Everything else — camera, orientation, geolocation, details, user menu,
   upload — is testable in dev.

3. **PRD 005 is not implemented**, so there is no install-first onboarding: "Add to
   Home Screen" is a manual Safari step, and the "Installed as app" row is
   deliberately absent (task 099).

## Acceptance Criteria

- [x] Capture works on both platforms — **confirmed on a real phone 2026-08-29:** both
      the in-app camera and the OS camera (`<input capture>`) upload successfully against
      the production-like stack at `hej.nathejk.dk`.
- [x] Orientation is correct — implied by the above: neither path produced a sideways
      portrait, which is what the server-side EXIF handling (task 104) was added for and
      the case the file-picker path would otherwise have failed.
- [x] Upload persists across a reload and on a second device — **confirmed on device
      2026-08-29** (reload).
- [x] Permission rows match real device state after changing it in settings — **confirmed
      2026-08-29**, tested with location.
- [ ] A previously denied notification permission can be recovered following only the
      on-page guidance. *Blocked on push being configured in the deployment — see
      Findings.*
- [x] Findings logged in this task; regressions filed as their own tasks.

## Findings

**2026-08-29 — notifications: "Tilmeld" does nothing.** Two separate problems, one of
them mine.

*The likely cause is configuration.* `enable()` fetches the VAPID public key and gives up
if it is empty; `docker-compose.prod.yml` defaults `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY`
to empty, so a deployment that never exported them cannot subscribe anybody. Confirm on
the host with:

```
curl -s https://hej.nathejk.dk/api/push/public-key
```

An empty `public_key` means push was never configured in this deployment — not a code
fault. Generate a pair (`npx web-push generate-vapid-keys`), export both, redeploy.

*The bug that made it hard to diagnose is mine.* The profile row never read
`notifications.error`, and `enablePush()` ignored `enable()`'s return value — so a failed
subscribe repainted the same "Tilmeld" invitation with no explanation. Exactly the shape
of "nothing happens when I tap it". Fixed:

- the store now records whether the **server** has push configured
  (`configured`), asked on page load rather than discovered inside a button press;
- when it is not configured the row says "Ikke klar — Nathejk har ikke sat notifikationer
  op endnu. Det er ikke din telefon, der er noget i vejen med." and offers **no button**,
  following the same rule as a blocked permission: never show an action that cannot work;
- any other failure shows the store's error text with a "Prøv igen" action, instead of
  silently repeating the invitation.

**2026-08-29 — "the photo stored are mirrored". Investigated, and kept as is.**

Not a bug in the stored bytes. `ctx.drawImage` ignores CSS transforms, so the capture is
the camera's **true** frame while the live preview is mirrored to feel like a mirror — the
two disagree, and it shows at the confirm step. The decisive evidence was the report that
*both* paths look the same: the OS-camera path never touches our canvas, and iOS also
saves the true image by default, so two independent paths agreeing points at expectation
rather than code.

Maintainer's decision: **leave it.** Mirroring what we store would match how people expect
to see themselves, but this photo exists so staff can identify a member in the dark, and a
mirrored image puts a parting, a scar, a badge or an arm number written on skin on the
wrong side for whoever is doing the identifying.

Recorded as a comment on `shoot()` in `PhotoCapture.vue`, because "the photo is mirrored"
is certain to be re-reported and the tempting fix is the wrong one. If it is ever
addressed, the correct change is to remove the mirror from the *preview* — not to add a
flip to the capture.

## Description

Runs after 107. (Task 102's consent blocker was cleared 2026-08-28.)

Camera, permissions and orientation cannot be verified in a desktop browser. Test
on **iOS Safari as an installed standalone PWA** and on **Android Chrome**
(baseline per `.rules`: iOS 16.4+ / Chrome 111+).

Cover: portrait capture in both orientations, front/rear flip, denying camera and
recovering via the guidance copy, the permission rows reflecting a change made in
system/browser settings after returning to the app, and the user menu's tap target
and dismissal behaviour on touch.

One thing was **de-risked in task 104** and needs confirming rather than
investigating: EXIF orientation is now read and applied **server-side**, so a photo
from the OS camera app via the `<input capture>` fallback should arrive upright
without the client doing anything. Proven against constructed EXIF headers in unit
tests; worth one real camera file.

Two more things to confirm rather than investigate:

- **HEIC.** iOS may hand a `.heic` file to the plain file input. Go cannot decode it,
  so the upload is rejected as "filen er ikke et billede vi kan læse". Confirm whether
  it happens in practice; if it does, the fix is a client-side canvas conversion in the
  fallback path (task 111's follow-up note), not a server-side decoder.
- **Storage growth.** Originals are now retained (task 111), 1–4 MB each against ~15 KB
  of renditions. Watch `/blobs` during the session — it is the one directory that must
  be backed up.

## Acceptance Criteria

- [ ] Capture works on both platforms, upload persists across a reload and on a
      second device.
- [ ] Permission rows match real device state after changing it in settings.
- [ ] A previously denied notification permission can be recovered following only
      the on-page guidance.
- [ ] Findings logged in this task; regressions filed as their own tasks.

## Progress Log

- 2026-08-28 — Task created from PRD 003 §10.
- 2026-08-28 — Prerequisites investigated and written up above rather than discovered
  with a phone in hand: DNS points the app's hostname at loopback, and the dev stack
  ships no service worker, so push is out of scope for a dev-stack session.
- 2026-08-28 — Two client fixes made in advance, both of which would have cost time in
  the session: the camera now starts on mount rather than during setup, and an insecure
  origin reports "kræver en sikker forbindelse (https)" instead of a generic "could not
  start" — the latter is exactly what someone reaching the stack over plain http on an
  IP would have seen while hunting for a permission problem that did not exist.

# Vendored front-end libraries for the admin tool

Pinned, committed, and served from the Go binary — **not** fetched from a CDN at runtime and **not** built.

## Why vendored rather than a CDN

The same three reasons the Leaflet vendoring already in this repo gives:

1. **The tool has to work on the Tuesday after the event**, on a hotel connection, possibly behind something
   that blocks a CDN. A curator with 300 photographs to hand in is not the person to discover that unpkg is
   unreachable.
2. **No supply-chain surprise.** A pinned file in the repo cannot change under us between the event being
   planned and the event happening. A CDN URL can.
3. **No build step.** Which is the whole point of this stack choice — see `roadmap/tasks/open/395-*.md` for why
   Tailwind was rejected in favour of Pico.

## Versions

| File | Library | Version | Source |
|---|---|---|---|
| `htmx.min.js` | htmx | **2.0.4** | `https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js` |
| `alpine.min.js` | Alpine.js | **3.14.9** | `https://unpkg.com/alpinejs@3.14.9/dist/cdn.min.js` |
| `pico.min.css` | Pico CSS | **2.0.6** | `https://unpkg.com/@picocss/pico@2.0.6/css/pico.min.css` |

`vendor.txt` holds the same table in a form a script can read, and
`TestTheVendoredAssetsMatchTheirPinnedVersions` checks each file still announces the version claimed here — so a
re-download that quietly picked up a different release fails the suite rather than shipping.

## Updating one

Re-download at the new version, update the table above **and** `vendor.txt`, run the suite, and check the two
admin pages by eye. There is no lockfile and no integrity hash: the file itself is the lock, because it is
committed.

```sh
curl -sS -o htmx.min.js   https://unpkg.com/htmx.org@<v>/dist/htmx.min.js
curl -sS -o alpine.min.js https://unpkg.com/alpinejs@<v>/dist/cdn.min.js
curl -sS -o pico.min.css  https://unpkg.com/@picocss/pico@<v>/css/pico.min.css
```

## Why Pico, not Tailwind

Recorded here because it is the question anyone will ask. Tailwind needs a build pipeline — its mechanism is
scanning markup and generating only the utilities used. The only Tailwind in this repo lives inside the **PWA's**
Vite build, and the maintainer's rule is that the PWA and the website stay separate:

> *"It's essential that we do not mix the pwa and the website, it's two different things and should be kept as
> such."*

So using Tailwind here would mean either a second pipeline for the website or coupling the two things that must
stay apart. Pico is one static stylesheet, mostly classless, form- and table-first — which is what this tool is.

## Why these are served behind the admin credential

They are public, unmodified libraries, so serving them unauthenticated would leak nothing. They are behind
`requireAdmin` anyway, because `/admin/*` is the admin surface and **every response from it carries
`Cache-Control: no-store`** (task 371) — a rule worth keeping true without exceptions more than worth saving a
download.

The cost is that ~180 KB is re-fetched per full page load. For two or three curators, a handful of loads a day,
that is nothing; the alternative was an exception to a security header, which is the kind of thing that is fine
until it is cited as precedent.

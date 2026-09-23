# Hej Nathejk

In-event companion app for Nathejk: maps, contacts, rulebook and event updates for
participants while the race is running.

- **Frontend** — Vue 3 + TypeScript PWA (`vue/`), served at
  `https://hej.local.nathejk.dk` in dev
- **BFF** — Go backend-for-frontend (`go/`), internal only; the frontend is its only
  client

Planning lives in `roadmap/`: product requirements in `roadmap/prd/`, work items in
`roadmap/tasks/`. Conventions for agents and contributors are in `.rules` and
`.agents/`.

## Running the dev stack

```sh
docker compose up -d
```

That brings up `ui`, `api`, `db` (MariaDB) and `phpmyadmin`. Both services hot-reload:
the frontend through Vite's HMR, the API through a watch loop that re-runs the gates
(test / vet / staticcheck / build) on every `.go` or `.sql` change and refuses to start
if any of them fail.

**HMR does not reach an app installed to a home screen.** The PWA uses
`registerType: 'prompt'`, so an installed client keeps serving its precached bundle until
the update prompt is accepted — deliberate, so a bundle never swaps under a participant
mid-race, and confusing during development, because the device shows old UI with nothing in
the logs to explain it. Reinstall the app (iOS) or *Clear & reset* the site (Android) to
refresh it. Full detail, including how to rule out the code in one command, is in the
`docker-dev-stack` skill.

**Testing on a real device needs more than the dev stack.** `hej.local.nathejk.dk` resolves
to `127.0.0.1`, which from a phone is the phone — and plain `http://<LAN-IP>` is not a
substitute, because the service worker, install prompt and geolocation all require a secure
context. Use a tunnel or a tailnet hostname; see `roadmap/tasks/open/172-offline-test-protocol.md`.

Two external Docker networks are expected, both owned by the `nathejk` repo:

- `traefik` — the reverse proxy that terminates TLS and routes the local hostnames
- `jetstream` — the shared NATS JetStream broker

The broker is deliberately **not** run by this repo. Every service in the org
(`hq`, `tilmelding`, `skan`, …) joins the same one, and a private broker here would mean
events `hej` publishes are invisible to everyone else.

If the API entrypoint (`docker/init/api-dev`) changes, rebuild rather than just
restarting — it is baked into the image:

```sh
docker compose build api && docker compose up -d api
```

## Where the data comes from

**Nothing writes to the database directly.** Every state change is an event on the
broker; SQL tables are projections rebuilt by replaying the stream. So there is no
migration step and no fixture loading: start the stack and the tables fill themselves.

That means a "reset" is just dropping the tables:

```sh
docker compose exec -T db sh -c 'mysql -uroot -p"$MARIADB_ROOT_PASSWORD" hej \
  -e "DROP TABLE IF EXISTS person, person_section, deadletter"'
docker compose restart api
```

The API recreates the schema and replays from sequence zero. A full replay of the
current dataset takes roughly two to three minutes; watch for it to finish with:

```sh
docker compose logs -f api | grep -E 'projections running|dead-letter'
```

`projections running, dead-letter queue empty` means the rebuild is clean. A non-zero
dead-letter count means some statement failed and was captured rather than killing the
consumer — inspect the `deadletter` table.

**A restart does not empty anything.** Projections are re-upserted over the existing
rows, so the app keeps serving the previous run's data while it catches up. Only an
explicit drop starts from nothing.

### Which event year the app reads

The member directory is keyed per event year and the API reads exactly one, set by
`EVENT_YEAR` (default: the current calendar year, pinned to `2026` in
`docker-compose.yml`). Participants from other years are projected but inert — they
cannot log in.

### Logging in locally

There is no password. `POST /api/auth/request-pin` sends an SMS containing a PIN — and in
dev the SMS sender only logs it:

```sh
docker compose logs api | grep 'kode er'
```

Or, faster: the dev panel shows the PIN for the number you just submitted — see
[Testing on this laptop](#testing-on-this-laptop-dev). It reads a dev-only endpoint
(`GET /api/dev/pin`) that is not registered outside `ENV=development`.

Pick any phone number that exists in the projection for the current `EVENT_YEAR`:

```sh
docker compose exec -T db sh -c 'mysql -uroot -p"$MARIADB_ROOT_PASSWORD" hej \
  -e "SELECT phone, name, appRole FROM person WHERE year=2026 AND deleted=0 AND phone<>\"\" LIMIT 5"'
```

(The password lives in the container's environment, so it has to be expanded *there* —
hence `sh -c` with single quotes rather than passing `-p"$MARIADB_ROOT_PASSWORD"`
directly, which would expand to nothing on your machine.)

Two things that surprise people:

- `request-pin` returns the same response whether or not the number is known. That is
  deliberate (anti-enumeration) — an unknown number simply never receives a PIN.
- Requests are rate limited to 5 per minute per IP. If PINs stop appearing while you are
  testing, wait a minute rather than debugging the directory.

### Testing on this laptop (`?dev=`)

The app is installed-mobile-only by design (PRD 005): a desktop browser is sent straight
out of the SPA to the anonymous website, and a phone in a browser tab gets the install
wall. That is the product behaviour, and it makes the dev machine the one device the app
refuses to run on.

So the dev build can **simulate a device** (PRD 014). Open the app root with `?dev=`:

```
https://hej.local.nathejk.dk/?dev=iphone
```

The choice is then remembered, so you can drop the parameter afterwards — it has to be
remembered, because the installed `start_url` is `/` and drops the query string.

**It must be an app URL.** `/desktop.html?dev=iphone` does nothing: that page is a plain
static file with no bundle, so there is no code on it to read the query. If you are looking
at the placeholder, edit the address bar back to `/?dev=iphone`.

| `?dev=` | What the app then believes it is |
|---|---|
| `iphone` | an iPhone, installed to the home screen |
| `ipad` | an iPad — which reports a *desktop* macOS UA, so this is the interesting one |
| `android` / `chromium` | an Android phone running Chrome, installed |
| `webview` | Facebook's in-app browser, where installing is impossible |
| `other` | a mobile browser that is neither WebKit-on-iOS nor Chromium (Firefox) |
| `tab` | the same platform, but **in a browser tab** — i.e. the install wall |
| `desktop` / `off` | stop simulating |

A **dev panel** (bottom-left, deliberately ugly) shows what is being simulated versus what
is real, and carries the rest of the controls:

- device presets, as above
- **login PIN** for the number you just typed, so you do not have to read the API log
- **fake position**, with accuracy, the five failure modes, and a walk that actually moves
- **simulated safe-area insets** (notch / home indicator), which a laptop otherwise has none of
- **force offline** — use this rather than DevTools, which kills Vite's HMR socket
- **resets**: onboarding, session, caches, service worker, or everything

#### `?dev=` is not `?nogate=1`

Worth being clear about, because they look interchangeable and are not:

- **`?nogate=1`** switches the install/device gates **off** — including the onboarding
  redirect. Use it to check that the `install_gate` kill switch works. It is *not* how to
  test onboarding, because it is the thing that skips onboarding.
- **`?dev=iphone`** leaves every gate **on** and changes what they see. Onboarding, the
  install wall and the role gates all run for real.

#### Web Push works here too

Desktop Chrome does real VAPID Web Push, so the whole notification flow — permission,
subscribe, `push-sw.js`, notification click — is testable on the laptop once you set
`VAPID_PUBLIC_KEY` and `VAPID_PRIVATE_KEY` in `docker-compose.override.yml` (generate a
pair with `npx web-push generate-vapid-keys`). The private key is a secret: it belongs in
the gitignored override file and nowhere else.

**iOS** push is the exception — it needs a home-screen web app on 16.4+, i.e. a real
phone. See `roadmap/mobile-only-checklist.md` for everything else that cannot be checked
from here.

### Seeding specific edge cases

The shared dataset is realistic but does not contain everything the app branches on —
and some of it not at all. Notably, no real crew member is assigned to a capability
section, so nobody in the real data has the `samarit`, `postmandskab` or `guide` role.

`cmd/seed` publishes synthetic events for those cases under a sentinel year (`9999`), so
they never mix with a real event:

```sh
# see what is available
docker compose exec -w /app api /usr/local/go/bin/go run nathejk.dk/cmd/seed -list

# publish them all (or one, with -case <name>)
docker compose exec -w /app api /usr/local/go/bin/go run nathejk.dk/cmd/seed
```

Then point the API at that year to use them — `EVENT_YEAR=9999`, e.g. via a temporary
compose override — and log in with the `+4599…` numbers it prints.

Seeded records are recognisable on sight: every name starts with `TEST ` and every phone
number with `+4599`. Note the seeder publishes to the **shared** broker and cannot
un-publish, which is why it refuses to run against a real event year.

### Photo albums (PRD 022)

The curator's own half-page — the credential, the year, and what delete means — is
[`docs/billedarkiv-for-fotografer.md`](docs/billedarkiv-for-fotografer.md). It is written for a volunteer
with a camera, not for a developer, and it is the thing to hand over with the password.

There is no album fixture. There used to be one — `/api/dev/album-fixture` — and it was deleted in task
383 once `/admin` could do the same job for real, because a fixture that publishes events in a shape the
projection has since changed is a generator of warnings rather than of test data.

So: open `https://hej.local.nathejk.dk/admin`, log in with `ADMIN_USER` / `ADMIN_PASSWORD` from
`docker-compose.yml`, drag some photographs in, create an album, select them, add them to it and tick
**Udgivet**. That is the whole loop, and it is the same loop a photographer and a curator use — which is
the point: the states worth looking at (an unpublished album, a coordinate outside the race area, an item
with no caption) are now states you produce the way a real user produces them.

Note `PUBLIC_ALBUMS=false` in dev: the albums section, the album pages, the map read and the album **media
bytes** all answer 404 until you turn it on.

### The map fixture world (PRD 016)

The map is the one feature whose states you mostly **cannot** produce by hand. A relative
open window needs an anchoring scan at another post; a skitse is handed over at a post and
has no QR code at all; a sheet reassigned to a successor team needs two teams and a
re-binding; and the state that matters most — a real, positioned checkpoint that no rule
reveals, which must therefore never appear — cannot be demonstrated by adding data, only by
checking that something is absent.

So when the API runs **without a broker** (the no-database mode), `/api/checkpoints` and
`/api/patrol/handouts` are served from `go/internal/mapfixture` instead of answering 503.
It fakes the five projections and runs the **real** `reveal.Rule` over them, so what you see
is what the reveal logic actually does. Log in as the mock spejder (`+4530000001`).

What the fixture patrol (`mock-patrol-1042`) has, and which state each thing exists to show:

| On the map / in the drawer | State it demonstrates |
|---|---|
| Post 1 (Silkeborg Sønderskov) | revealed by a QR-bound sheet **and** by a scanned checkgroup; `fixed` window |
| Post 2 (Kløvermarken) | `relative` window, anchored on the Post 1 scan — a verdict that is only computable because we hold the anchor |
| Post 3 (Ans Bro) | revealed by a **skitse** handed over at Post 2's line; its handout is *synthesised* (no sticker number) |
| Post 4 (Gjern Bakker) | revealed by a sheet since **reassigned away** — revealing is monotonic, so it stays revealed; `none` scheme, so no verdict |
| Post 5 | revealed but **not sited** (no position): drawn nowhere, arrowed at nothing |
| Post 9 ("hemmelig") | **revealed by nothing.** If this ever appears on the map, that is the bug PRD 016 exists to prevent |
| "Etape 1", nr. 1042 | a QR-bound sheet still held |
| "Etape 4", nr. 1055 | **"afleveret"** — and it names no other team, deliberately |
| "Ukendt kort", nr. 9999 | a code registered before its sheet was recorded: unknown sheet, not no sheet |
| "Skitse til etape 3" | a synthesised handout, with no sticker number and no gap where one would go |

The verdict badges in the drawer come from `internal/scans`' mock, which covers on time, late
with a delta, early, no window, an unattributable registration and a bandit catch. Its
checkpoint ids match the fixture above so scanned posts show as visited — **change one and
change the other.**

The fixtures cannot reach production. They are chosen by the *absence of eventing*, never by
a flag, and a broker whose projections failed to build still gets an honest 503 rather than a
plausible-looking map (see `mapReadsFor` in `cmd/api/mapsource.go`).

## Tests and checks

The same gates the dev loop runs, from `go/`:

```sh
cd go
go test ./... && go vet ./... && go tool staticcheck ./... && go build ./...
```

Production builds compile with `GOWORK=off` (the workspace and its `shared-go` mount do
not exist in the image), so it is worth running the gates both ways before pushing:

```sh
GOWORK=off go build ./... && GOWORK=off go test ./...
```

**The dev loop now checks this for you** (`docker/init/api-dev` runs a `GOWORK=off go build`
as its last gate), because relying on the instruction above was not enough — CI broke on
exactly this in 2026-09. The failure mode is worth understanding, since it is invisible
locally: `go.work` makes the sibling `../shared-go` checkout live, so code using a symbol
that exists only there compiles and tests perfectly on your machine and fails the moment CI
resolves the version pinned in `go.mod`.

When that happens the fix is two steps, in order:

```sh
# 1. push shared-go, then
cd go && go get github.com/nathejk/shared-go@latest && go mod tidy
```

A `hej` change that needs a `shared-go` change is therefore never a single-repo commit — the
dependency has to land upstream first.

Frontend type-checking runs in the container, since there is no host Node:

```sh
docker compose exec -T ui npx vue-tsc --noEmit
```

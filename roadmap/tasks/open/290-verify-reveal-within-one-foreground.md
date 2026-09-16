# 290 — Verify on device: a reveal appears within one foreground

**Status:** open
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 017 phase 1's acceptance test, and the one that cannot be faked in a unit test:
the happy path from §5, end to end, on a real phone.

**Needs a device and a way to trigger a handout/reveal server-side. Not completable by
an agent session.**

The scenario:

1. A patrol holds Kort 1–2; note what the map draws and what the drawer lists.
2. Lock the phone.
3. Server-side, hand the patrol Kort 3 (and let the reveal rule expose its checkpoints).
4. Unlock the phone and return to the app **without** force-quitting it.
5. The new checkpoints must appear, and the new sheet must be listed, within one
   foreground — no cold start, no pull, no waiting for the 60 s tick.

Then the same for a scan: scan a post, lock, unlock, and the drawer must gain the row.

Why this is worth a task of its own: every part of the mechanism can be green in tests
while this fails, because what makes it work is whether the *resume* fires an event the
loop hears (task 280). This is the difference between the feature working and appearing
to work.

## How to run it

### 0. What this is actually testing now

Task 280 has since measured the trigger half on this device: ten resumes, every one firing
`visibilitychange` → visible, so the loop checks on foreground. What remains untested is the rest of
the chain — **event → projection → version → refetch → UI** — with real data. So if this fails, the
likely culprit is the data path, not the resume. There is a two-second way to tell them apart; see
step 5.

**Before starting, check the device is on the build you think it is** (task 298): the app checks for a
new build only once per document load, so an already-open app can be hours behind. Compare the build id
on the bottom nav against `curl -s <host>/api/healthcheck | jq .system_info.version`. Testing a
mechanism on a build that predates it is the most expensive way to get a false negative here.

### 1. Sign in as a real patrol member

**Which datasets you can test depends entirely on who you are signed in as**, and that is the design
rather than an obstacle. A crew/personnel account has **no patrol**, so `/api/sync` deliberately omits
`scans`, `handouts` and `checkpoints` — there is no scan drawer to watch and no reveal to trigger. It
does have `contacts` and `profile`.

So:

| signed in as | testable datasets | how to change them |
|---|---|---|
| **spejder with a patrol** | scans, handouts, checkpoints (+ profile) | `cmd/simscan`, skan QR binding |
| **crew / personnel** | contacts, profile | edit a member in the register, or add a portrait |

The crew path is worth doing even though it is not the headline scenario: it exercises the identical
chain — event → projection → version → refetch → UI — and needs no simulation tooling at all. Change a
phone number or add a portrait for somebody in the contact list, lock, wait, unlock: the row must
change. If that works, the mechanism works, and the patrol-scoped datasets differ only in which
projection moved.

**A free prediction to check while you are there:** as crew, `/api/sync` must contain `contacts`,
`profile` and `race_area` and **must not** contain `scans`, `handouts` or `checkpoints`. Absence is how
the client learns not to ask (PRD 017 §6). If you can see the response, that is a one-glance
confirmation of the permission model.

The dev API starts on the mock directory but **switches to the real person projection as soon as a
database is present** (`main.go`, `newSwitchableDirectory`). The mock's patrol ids do not exist in the
projections, so a mock login cannot be used for the patrol-scoped half: `simscan` and skan publish
against real team ids.

Use a real registered number, and read the PIN from the dev endpoint rather than the logs:

```sh
curl 'https://hej.local.nathejk.dk/api/dev/pin?phone=30001234'
```

(dev-only route; 404 outside `ENV=development`.)

### 2. Find the ids you will need

From phpMyAdmin (already in the compose stack) or straight from the db container:

```sh
docker compose exec db mysql -uroot -p"$MYSQL_ROOT_PASSWORD" nathejk -e "
  SELECT teamId, number, name FROM team WHERE year='2026' LIMIT 5;
  SELECT checkpointId, name, checkgroupId FROM checkpoint WHERE year='2026' LIMIT 10;
  SELECT code, teamId, kortId, lastUts FROM maphandout WHERE year='2026' LIMIT 10;"
```

You want the signed-in member's `teamId` and its printed `number`, plus a `checkpointId` in a
checkgroup the patrol has **not** already had revealed.

### 3. The scan half — `cmd/simscan`

```sh
cd go
go run ./cmd/simscan -checkpoint <checkpointId> -team <teamId> -team-number <number>   # dry run
go run ./cmd/simscan -checkpoint <checkpointId> -team <teamId> -team-number <number> -confirm
```

Read that command's header before using it. In short: it is a **dry run without `-confirm`**; it
publishes three facts (a synthetic postmandskab crew member, a `checkpersonnel` shift covering now,
and the `qr.scanned`); the stream is **append-only** and the broker is **shared with hq, tilmelding
and skan**, so what you publish is permanent and visible to them too. The scan's timestamp is the
publish time and cannot be backdated, so against a real event window the verdict may legitimately
read "for tidligt" — irrelevant here, since the test is whether the row *appears*.

### 4. The handout / reveal half — the skan app

There is no `simhandout`, and the right tool already exists: **the physical map ↔ QR ↔ team binding is
made in skan** (see this projection's own header). So bind a sheet's QR code to the patrol in the skan
app, exactly as a post would. That publishes `qr.registered`, hej's `maphandout` projection consumes
it, and the reveal rule exposes that sheet's checkpoints.

Using skan rather than a new injector is deliberate: it exercises the real producer, so a mismatch
between what skan publishes and what this app expects is *in scope* for this test rather than papered
over by a tool written against our own reading of the schema.

### 5. The measurement, and the one way to get a false negative

1. Open the app on the map, note what the drawer lists and what the map draws.
2. **Lock the phone** — do not force-quit. A cold start passes trivially via the mount check and
   proves nothing about resume, which is the entire point of this task.
3. Publish the scan (or bind the QR).
4. **Wait at least 5–10 seconds before unlocking.** This is the false negative to avoid: the BFF
   caches each version for 5 s, so a check that lands within that window legitimately returns the
   *old* version and refetches nothing. Unlocking too fast looks exactly like the feature being
   broken.
5. Unlock. The new row (and any newly revealed posts) must appear **without** pulling, tapping
   anything, or waiting out the 60 s tick.

**If nothing appears, tap Opdatér before concluding anything.** That splits the failure in two:

- it appears on tap → the data path is fine and the **trigger** failed (contradicting task 280 — worth
  reopening it with the probe at `/genoptag`);
- it still does not appear → the **data path** is where to look: check `maphandout`/`scan` actually
  projected, then whether the version moved (the BFF logs `sync dataset summary` every five minutes
  with per-dataset churn).

## Acceptance Criteria

- [ ] Reveal: new checkpoints drawn within one foreground, from a locked phone. *(needs a spejder
      login)*
- [ ] Handout: new sheet listed within one foreground. *(needs a spejder login)*
- [ ] Scan: new row in the drawer within one foreground. *(needs a spejder login)*
- [x] **Contacts: a changed row appears within one foreground.** — confirmed on device 2026-09-17: a
      portrait changed server-side appeared on unlock, with no prompt and no tap.
- [ ] Confirmed as crew that `/api/sync` omits `scans`/`handouts`/`checkpoints` and carries
      `contacts`/`profile`/`race_area`. *(the absent scan drawer is consistent with it, but the response
      itself was not inspected — not ticking what was not looked at)*
- [ ] Same three, returning from the app switcher rather than from lock.
- [ ] Same three, returning after a bfcache navigation (external link and back).
- [ ] Confirmed no duplicate `/api/sync` per resume (debounce working, task 281).
- [ ] Confirmed no payload requests when nothing changed.
- [ ] Network trace or screenshots in the Progress Log.

## Progress Log

- 2026-09-16 10:00 — Task created from PRD 017 phase 1. Blocked on device access and a
  server-side handout trigger.
- 2026-09-17 02:15 — Wrote the procedure above rather than leaving "needs a device" as the whole
  instruction. Three things worth knowing that were not obvious before looking:
  1. **A mock login cannot be used.** The dev API switches to the real person projection as soon as
     a database is present, and the mock directory's patrol ids do not exist in the projections — so
     a scan published for a real team id would never match the signed-in user. This is the most
     likely way to waste an evening on this task.
  2. **`cmd/simscan` already covers the scan half** (task 270's tooling), including the shift and
     crew member a scan needs in order to attribute to a post at all.
  3. **The handout half should go through skan, not a new injector.** The QR ↔ team binding is
     skan's job by design, and using the real producer keeps a schema mismatch between the two apps
     inside the scope of this test instead of papering over it with a tool written against our own
     reading.
- 2026-09-17 02:20 — Also recorded the **5 s version-cache window** as an explicit false-negative
  trap: publish, then wait before unlocking, or the check correctly returns the cached old version
  and the feature looks broken. And the **Opdatér-button differential** — if the data appears on a
  manual tap, the trigger failed; if it does not, the data path did. That turns a "it didn't work"
  into one of two much smaller questions.
- 2026-09-17 02:35 — The device available for this is signed in as **crew, not a spejder**, so there
  is no scan drawer and nothing patrol-scoped to trigger — `/api/sync` legitimately omits those three
  datasets for a user with no patrol. Added a **contacts-based route to the same verification**, which
  needs no simulation tooling at all and exercises an identical chain; the patrol-scoped criteria stay
  open for a spejder login. Also added the crew-omits-patrol-datasets check, which is a free
  confirmation of the permission model while somebody is looking at a response.
- 2026-09-17 02:40 — Noted task 298 at the top: that device was on `main.85` against `main.87`, so any
  measurement taken now needs the build id checked first. Testing this mechanism on a build that
  predates it would be the most expensive possible false negative.
- 2026-09-17 04:00 — **The mechanism is confirmed end to end.** On `main.90`, a portrait changed
  server-side appeared in the contact row on unlock — no prompt, no tap, no wait for the interval. That
  exercises every layer this PRD touches: event → projection → version derivation → `/api/sync` → version
  comparison → manifest refetch → portrait cache-bust → render.

  What that leaves is **dataset-specific coverage, not mechanism risk**: scans, handouts and checkpoints
  differ from contacts only in which projection moved and which store refreshes. Both remaining layers
  have unit coverage (`syncmetrics_test.go`, `mapversion_test.go`, `syncVersions.spec.ts`), so the
  residual risk is a wiring mistake in one of three dispatch entries rather than a design fault.

  This task therefore stays open, deliberately, as **verification work that outlives PRD 017** — it needs
  a spejder login and either `simscan` or a QR binding in skan, neither of which is available on the
  device that ran this. It is no longer blocking the PRD.

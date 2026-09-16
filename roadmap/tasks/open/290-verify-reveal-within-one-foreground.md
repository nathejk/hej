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

### 1. Sign in as a real patrol member

The dev API starts on the mock directory but **switches to the real person projection as soon as a
database is present** (`main.go`, `newSwitchableDirectory`). The mock's patrol ids do not exist in the
projections, so a mock login cannot be used here: `simscan` and skan publish against real team ids.

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

- [ ] Reveal: new checkpoints drawn within one foreground, from a locked phone.
- [ ] Handout: new sheet listed within one foreground.
- [ ] Scan: new row in the drawer within one foreground.
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

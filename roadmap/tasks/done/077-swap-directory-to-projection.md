# 077 — Swap users.Directory to the projection

**Status:** done
**Priority:** high
**Created:** 2026-08-25
**Picked up by:** agent
**Started:** 2026-08-26
**Completed:** 2026-08-26

## Description

PRD 006 §6. Make the real projection satisfy `users.Directory` so login and
role-gated navigation read real data. The interface is the seam PRD 001 designed
for exactly this, so handlers must not change.

Keep `users.NewMockDirectory()` as the **test double** — it should not be deleted,
and it must not grow into a data source (PRD 003's task list was corrected on this
point).

Preserve the anti-enumeration contract: `found=false` must be indistinguishable
from `found=true` to the client, so verification simply never succeeds for an
unknown number.

## Acceptance Criteria

- [x] An adapter satisfying `users.Directory` backed by the person querier
- [x] Wired in `main.go`; mock retained for tests
- [x] Falls back to the mock (or fails clearly) when there is no database
- [x] No handler signature changes
- [x] Anti-enumeration behaviour unchanged and tested
- [x] Full suite green on both workspace and GOWORK=off paths

## Progress Log

- 2026-08-25 — Task created from PRD 006.
- 2026-08-26 — Done. **Real members can now log in.** Verified end to end against the
  live projection, including the negative cases.

  **Two things the interface does not carry, which the adapter had to supply.**

  1. **The year.** `users.Directory` is phone-in/user-out; the projection is keyed per
     event year. Introduced `EVENT_YEAR` (default: the current year), pinned to `2026` in
     the dev compose file. Deliberately not `time.Now().Year()` the way hq does it — the
     clock is wrong for a test event held outside its nominal year, and wrong in the days
     around new year, when the app would stop recognising every participant of an event
     that has not happened yet. Answers PRD 006 §11 Q7. Fixed once at construction, not
     read per call, so every lookup in a request agrees about which event is running.
  2. **Role translation.** `person`'s app-role strings are deliberate duplicates of
     `users.Role`'s values, so this is a conversion and not an inference — nothing here
     re-derives a role from a team type or a section slug. A test walks `users.AllRoles`
     through the adapter, which is what will catch the two lists drifting apart; the
     symptom otherwise is a real member being refused a login.

  **An unrecognised role refuses the login.** Neither alternative was acceptable: passing
  the string through puts an unknown value in the session and in `GET /api/me`, where the
  frontend router guard compares against a fixed enum and would show the wrong navigation;
  defaulting to `RoleCrew` would *grant* access off the back of a data problem, and
  `RoleCrew` means "crew whose section we could not classify", which is a specific
  understood condition that must not lend its privileges to "we have no idea what this
  row is". Logged, since from the client's side it is indistinguishable from an unknown
  number. On current data it never fires — all 3,278 rows carry a known role.

  **`switchableDirectory`, and why the indirection is forced.** The projection is built
  inside the broker's connect callback (deliberately, so a database-only run does not
  create tables nothing will fill), and that callback runs on a goroutine *after* the HTTP
  server is assembled and possibly already serving. Handlers therefore hold a
  `users.Directory` that does not exist yet. The alternatives were to build the projection
  eagerly — reintroducing the table problem and letting a slow broker delay the API — or
  to have handlers check for nil, spreading a startup concern across every call site. An
  atomic pointer, because the read path is every authenticated request and the write path
  happens once. Exercised under `-race`.

  **There is no empty-directory window, and the reason is load-bearing.** I nearly gated
  the swap on a caught-up signal, then checked: nothing truncates the `person` table on
  boot (no `TRUNCATE`/`DELETE FROM` anywhere outside tests). A restart therefore serves
  the *previous* run's rows while the replay re-upserts them, so no member is ever wrongly
  told their number is unknown — this is PRD 008 §5's "reads served from existing
  projections" in practice. The comment at the swap site records that a future change
  which does truncate on boot must move this behind `CatchupListener`.

  ### Live verification against the real stream

  Every step below used a real member's number from the 2026 data, through Traefik over
  HTTPS:

  | check | result |
  |---|---|
  | `request-pin` for a real member | PIN issued to `+4520351385` |
  | `verify` | session for `efdeaf50-…`, role `spejder` — matches the projected row exactly |
  | `GET /api/me` on the session | same user — so `Get()` by id resolves through the projection too |
  | **real guardian number** (`+4560634039`, nobody's own phone) | **no PIN issued** |
  | unknown number `+4599999999` | no PIN |
  | all three responses | byte-identical — anti-enumeration holds |
  | real shared number (`+4523936031`, two gøgler siblings) | two candidates, first names + group, **no session cookie** |
  | choose one candidate | session as `gøgler`, `/api/me` agrees |
  | replay the token with a `user_id` from a **different** phone | **401** |

  Two things worth drawing out. The guardian-number check is the live proof of the
  boundary decided this morning, on real data rather than a unit test. And the chooser's
  `team` field is populated only because task 075 put the gøgler scout group in
  `teamName` — without it these two siblings would have been offered as two
  indistinguishable rows.

  A `wget` attempt at `/api/me` over plain HTTP failed first; that was the `Secure`
  cookie behaving correctly, not a bug. My first `choose` call also failed with
  `unknown field "choice_token"` — the field is `token`; the request was wrong, not the
  handler, and the strict decoder rejecting it is the desired behaviour.

  **The mock is retained**, as the initial value of the switch and as the test double in
  `routes_test.go`. A run with no database or no broker simply stays on it, which is the
  documented degraded mode and keeps `hej` runnable at every step. No handler signature
  changed.

## Files

- `go/cmd/api/directory.go` (new) — `personDirectory` adapter + `switchableDirectory`
- `go/cmd/api/directory_test.go` (new)
- `go/cmd/api/env.go` — `eventYear` / `EVENT_YEAR`, `currentYear()`
- `go/cmd/api/main.go` — build the switch, install the projection once running
- `docker-compose.yml` — `EVENT_YEAR: "2026"` for dev
- `roadmap/prd/doing/006-member-directory.md` — §11 Q7 answered; 077 ticked

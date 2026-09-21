# 359 — Hide the curated photo albums until there are photographs

**Status:** done
**Priority:** high
**Created:** 2026-09-21
**Picked up by:** agent session (Zed)
**Started:** 2026-09-21
**Completed:** 2026-09-21

## Description

Maintainer instruction, 2026-09-21, preparing the first production deploy of the public site:

> hide the top photo album on frontpage /2026
>
> hide both, we have no photos at the moment - i want to get the rest in prod

The patrol pages, the patrol lookup and the glimt strip are ready to ship. Curated albums are a section with
nothing behind them: the two albums on the frontpage were the dev fixtures (`/api/dev/album-fixture`).

### Hidden, not empty — the distinction is the whole task

The frontpage already had an empty state for this section: *"Der er ikke lagt billeder op endnu."* Reaching for
it would have been the obvious wrong fix. That sentence is a **promise with a date on it** — it says photographs
are coming and invites a return visit. There is nothing to promise yet.

So `PUBLIC_ALBUMS=false` means the feature is not part of the site:

- No section, no heading, no empty box.
- **The intro line changes.** It read *"Billeder fra løbet, og patruljernes egne sider med deres rute"* — a page
  that opens by advertising something it does not have. It now leads with what it does have.
- **`/{year}/album/{slug}` answers 404.** A section removed from a page whose links keep serving is not hidden,
  it is unadvertised: old links, shared messages and indexes all still reach it. 404 rather than the 503 the
  missing-projection case returns, because those mean different things — 503 says "come back later", 404 says
  "there is nothing here", and a crawler should be told the second.

### Why a flag rather than deleting the section

Because it comes back, and the value of configuration over a code change is exactly that: turning the albums on
when the photographs are curated is an env change and a restart, not a release.

It defaults **on**, like every flag in `env.go`, for the reason recorded there about `install_gate`: a feature
that is off by default is a feature nobody tests. The albums are shipped, tested code. Production turns them off
(`docker-compose.prod.yml`), and the dev compose does too — because the ask was to hide them *now*, and a dev
environment still showing fixture albums would hide the change from the only person looking at it.

`newTestApp` sets it **true**, mirroring the production default rather than Go's zero value. A suite running with
the flag off would have been testing the hidden state everywhere and the feature nowhere.

### Not touched: the glimt strip

"Both" was read as *both albums*, which the section's removal covers. The glimt section stays, and its empty
state is honest in a way the albums' was not: glimt are shared by participants during the race, so *"Der er ikke
delt nogen offentlige billeder endnu"* is a true statement about the event rather than an unkept promise. If it
should also be hidden for the deploy, it wants the same treatment and it is ten minutes.

## Acceptance Criteria

- [x] The frontpage renders no album section and no empty state when the flag is off
- [x] The intro does not promise photographs that are hidden
- [x] An album page answers 404 while the feature is hidden
- [x] The lookup and the glimt strip are unaffected
- [x] Switching it back on restores both
- [x] Verified by breaking it: all three "hidden" tests fail if the handler ignores the flag
- [x] Off in `docker-compose.prod.yml` and in the dev compose, overridable per environment

# 370 — No password, no routes: the admin surface is absent when unconfigured

**Status:** open
**Priority:** high
**Created:** 2026-09-22
**Picked up by:**
**Started:**
**Completed:**

## Description

If `ADMIN_PASSWORD` is unset, the admin routes are **not registered at all** — `/admin`,
`/admin/album/:slug` and every `/api/admin/*` path answer the router's own 404 because no handler
exists. Implemented in `go/cmd/api/routes.go` behind an `adminRoutesEnabled(cfg)` predicate in the
same shape as `devRoutesEnabled` in `go/cmd/api/dev.go`.

The distinction PRD 022 §8.2 insists on is between *absent* and *open*. A default credential, or a
guard that lets everything through when the config is empty, means a misconfigured deploy publishes an
**unprotected bulk upload endpoint** on the public service — an anonymous write path into the one
store in this service that cannot be rebuilt from the log. `dev.go`'s comment already states the rule
this borrows: the check is "is this route registered", not "is this caller allowed", so there is no
handler left behind to be reached by a misconfiguration.

The second reason is operational and is PRD 022 §8.11's last bullet: **the tool is unattended between
events.** It will be used hard for a week and then not at all for a year, which is exactly the
situation in which a stale or unprotected deployment goes unnoticed. Turning the tool off is therefore
deleting the password, which is a thing somebody will actually do.

Note the asymmetry with `PUBLIC_ALBUMS` (task 359), and keep it: the admin tool is **not** gated by
that flag (PRD 022 §6), because a curator must be able to assemble albums before the public section is
switched on. Two different switches for two different questions.

## Acceptance Criteria

- [ ] With `ADMIN_PASSWORD` empty, every admin path answers 404 and the handlers are not reachable by
      any header, method or casing — tested by enumerating the admin route table
- [ ] There is no default, fallback or dev-only admin password anywhere in the tree
- [ ] With the password set, every admin route is registered and behind `requireAdmin`
- [ ] Startup logs once, plainly, whether the admin tool is enabled or absent
- [ ] `PUBLIC_ALBUMS=false` does not disable the admin tool — tested
- [ ] A test fails if a new admin route is added outside the conditional block

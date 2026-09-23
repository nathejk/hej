# 370 — No password, no routes: the admin surface is absent when unconfigured

**Status:** done
**Priority:** high
**Created:** 2026-09-22
**Picked up by:** agent session (Zed)
**Started:** 2026-09-23
**Completed:** 2026-09-23

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

## What landed

`adminRoutesEnabled(cfg)` in a new `admin.go`, mirroring `dev.go`, with the routes registered inside the
conditional. Shipped with tasks 369 and 371 — see 369 for why the three are one commit.

**Keyed on the password alone, not the username**, and that asymmetry is deliberate: a username is not a
secret, and a deploy that set only `ADMIN_PASSWORD` would otherwise serve a tool whose username is the empty
string. With the password set and no username the credential is `"" / <password>`, which is a real credential
and checked as one.

**`ENV` is deliberately not part of the predicate.** Unlike the dev routes, this is a production feature — it
is where the event's photographs actually arrive — so gating it on the environment would gate the wrong thing.

A second, cheaper line of defence sits behind the gating: `adminCredentialOK` refuses outright when the
configured password is empty, rather than matching an empty presented one. It should be unreachable, and it is
checked because "unreachable" is a property of today's `routes()`. That layering showed its worth during the
break-test: with the gating deliberately sabotaged to always-on, the surface answered 401 rather than 200.

## One thing worth knowing about the 404

With no password, `/admin` does not answer a bare 404 — it falls through to `spaHandler`, which serves
`index.html` with a **200** so a client-side route survives a reload. So "absent" means *the tool is not
served*, not *the path 404s*.

That caught out the first version of two tests, which asserted on status and would have passed for the wrong
reason. Both now assert on a content marker the tool renders and nothing else does. Worth recording because
anybody testing this surface will hit the same trap.

## Verified live, twice

Against the dev stack through Traefik, before and after configuring the credential: with no `ADMIN_PASSWORD`
the SPA shell answers and the tool's marker is absent; with it set, the tool renders and the startup log says
so. And a genuine dev-loop bug surfaced: **Vite's proxy did not forward `/admin`**, so the tool was
unreachable in a dev browser and looked broken rather than unrouted — the same gap the public pages' proxy
comment already warns about. Fixed in `vue/vite.config.ts`.

## Acceptance Criteria

- [x] With `ADMIN_PASSWORD` empty, no admin path serves the tool — tested by enumerating the admin route table
      **from source**, with three credential variants each, and asserting on content rather than status
- [x] No spelling of the path skips the credential. httprouter's `RedirectFixedPath` canonicalises case, so
      `/Admin` 301s to `/admin` — which is fine, because the guard is on the handler, not the spelling. The
      test was reshaped to assert that rather than a case-sensitivity that was never true
- [x] There is no default, fallback or dev-only admin password anywhere in the code — asserted against
      `env.go`'s flag registration and against four files for assignments
- [x] With the password set, every admin route is registered and behind `requireAdmin`
- [x] Startup logs once, plainly, whether the tool is enabled or absent — `Info` either way, because no admin
      password is a good production posture for most of the year and a `Warn` would train people to ignore it
- [x] `PUBLIC_ALBUMS=false` does not disable the admin tool — tested, and the same test asserts the public
      side is still hidden so it cannot pass by the flag having broken
- [x] A test fails if a new admin route is added outside the conditional block

# 466 — An absolute-URL helper for the public site

**Status:** done
**Priority:** high
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

**PRD:** 026

## Description

`og:url` and `og:image` must be absolute — a relative value is silently ignored by Facebook — and the service has no
notion of its own origin today. `publicRoot()` returns `/2026`.

**Derived from the request, not configured.** The scheme comes from `r.TLS` or `X-Forwarded-Proto` exactly as
`adminTransportOK` already does, and the host from `r.Host`. That header is only trustworthy because our own proxy is
the sole route to this service, which is the comment `middleware.go` already carries and which must be repeated here.

The alternative — a `PUBLIC_BASE_URL` config — was rejected in PRD 026 §8: it can silently be wrong in production and
break every preview at once, while a wrong `Host` can only come from a request that was already sent somewhere odd.

`renderPublicPage` has no `*http.Request` today, so it needs one — which is the right shape anyway, since the origin
and the default card should be filled by the renderer rather than by eight handlers remembering.

#
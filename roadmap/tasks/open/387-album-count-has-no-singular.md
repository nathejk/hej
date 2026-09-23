# 387 — "1 billeder" on the frontpage: the album count has no singular

**Status:** open
**Priority:** low
**Created:** 2026-09-23
**Picked up by:**
**Started:**
**Completed:**

## Description

The frontpage album card renders its photograph count with no singular form:

```
{{if .Count}}<span class="meta">{{.Count}} billeder</span>{{end}}
```

— `go/cmd/api/publicsite.go`, the `Albums` range in the frontpage template. An album holding one
photograph therefore reads **"1 billeder"**, which is wrong Danish. It should be "1 billede".

Found while verifying task 386 live: with `PUBLIC_ALBUMS=true` the dev frontpage showed "2 billeder" for one
album and "1 billeder" for another.

Small, and worth doing rather than shrugging at, for two reasons. It is on the **public** page — the one
surface read by families rather than by organizers — and a curator will hit it on the first album they build,
because an album starts with one photograph in it. It is also the kind of thing that reads as carelessness
about the whole page.

## Where else to check

Do not fix only the one line. The same construction is likely elsewhere, and a fix that leaves siblings
wrong is worse than none because it makes the remaining ones look deliberate:

- `publicsite.go`'s album card (the reported one).
- The album page itself, and the glimt strip's counts if any.
- `cmd/api/adminpage.go` and `adminalbumpage.go` — the curator's counts (`Kontaktark`, the album editor's
  item count, the selection counters, the delete confirmation's "you are about to delete N"). The delete
  confirmation matters most: task 379 made a point of the count being the thing a curator reads before
  confirming, and "1 billeder skal slettes" undercuts that.

A helper rather than an `{{if}}` at each site, since there will be more of these — but only if there are
genuinely several. One `{{if eq .Count 1}}` is not worth a helper.

## Acceptance Criteria

- [ ] An album with one photograph reads "1 billede" on the frontpage
- [ ] Every other count in the public site and the admin tool checked, and any with the same fault fixed
- [ ] A test covers the singular, not only the plural — the existing frontpage tests assert "2 billeder" and
      would pass with the bug intact
- [ ] Verified live on the dev frontpage with a one-photograph album

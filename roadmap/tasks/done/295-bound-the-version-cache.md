# 295 — Bound the version cache: its key space is no longer role combinations

**Status:** done
**Priority:** medium
**Created:** 2026-09-16
**Picked up by:** agent session (Zed / Claude)
**Started:** 2026-09-16
**Completed:** 2026-09-16

## Description

`versionCache` (`go/cmd/api/contacts.go`) never removes an entry. Its doc comment says
that is fine, and gives the reason:

> Not an LRU because the key space is bounded by the number of role combinations, which is
> fixed by the access matrix.

That was true when the cache had one user: `contactsVersionFor` keys by permitted role set,
of which there are three or four for a whole event. **It stopped being true in tasks 269 and
283**, which added four more caches on the same type:

| cache | key | live key space |
|---|---|---|
| `contactsVersions` | permitted role set | 3–4 |
| `checkpointsVersions` | patrol + started-state | hundreds |
| `handoutsVersions` | patrol | hundreds |
| `scansVersions` | patrol | hundreds |
| `profileVersions` | **user id** | thousands |

So `profileVersions` grows one entry per user who ever foregrounds the app, and nothing ever
takes one out. The entries expire — `get` refuses them after the TTL — but expiry is not
removal, so the map only ever grows for the life of the process.

The cost is not dramatic (a few thousand short strings), which is exactly why it would
survive a release: it is a slow leak in a long-running process, on the endpoint every device
calls on every foreground. It is worth fixing before an event rather than discovering the
shape of it during one.

**The false comment is the more serious half of this.** A justification that no longer holds
is worse than no justification: the next person adding a cache reads "bounded by the access
matrix", keys theirs by patrol, and is not wrong to think they followed the rule.

## Approach

Sweep expired entries when the map has grown past a threshold. Deliberately **not** an LRU
and deliberately not a background goroutine:

- Only *expired* entries are dropped, so a sweep can never evict an answer that is still
  being served — correctness is unchanged whatever the threshold is.
- The remaining size is then bounded by the number of *distinct callers within one TTL*,
  which is the real working set rather than the historical one.
- A goroutine would need a lifecycle, and this needs to hold no more than five seconds of
  callers.

## Acceptance Criteria

- [x] Expired entries are removed rather than accumulating.
- [x] A live entry is never evicted, whatever the threshold.
- [x] The doc comment states the real key space per cache, and why the bound now holds.
- [x] Test: a cache driven past the threshold with expiring keys stays bounded.
- [x] Test: a live entry survives a sweep triggered by other keys.
- [x] Test: many live entries at once are all still served (no cap on correctness).
- [x] All four Go gates green.

## Progress Log

- 2026-09-16 18:00 — Task created while instrumenting `/api/sync` for task 293. Found by
  reading the cache's own comment and noticing it described a key space two tasks had since
  changed.
- 2026-09-16 18:15 — Fixed. `put` sweeps expired entries once the map reaches
  `versionCacheSweepAt` (512), before inserting, so a cache that only grows through that path
  cannot outrun the sweep. The doc comment now carries the **table** of key spaces per cache,
  which is the part that would have prevented this: the next person adding a cache can see at
  a glance that "bounded by the access matrix" describes only the first row.
- 2026-09-16 18:20 — **The design point worth keeping:** a sweep drops only *expired* entries,
  so it can never evict an answer still being served. That means correctness does not depend
  on the threshold at all — only memory does — which is what makes 512 a number nobody has to
  defend. Three tests pin exactly that: bounded under many sequential callers, a live entry
  surviving a sweep triggered by other keys, and 612 simultaneously-live entries all still
  served past the threshold.
- 2026-09-16 18:25 — Completed. All four Go gates green. Note for task 293: churn detection
  cannot piggyback on lingering expired entries any more, since they are now removed — it needs
  its own bounded witness sample.

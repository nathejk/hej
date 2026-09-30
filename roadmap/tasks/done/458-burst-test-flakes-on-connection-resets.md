# 458 — The morning-after burst test flakes with connection resets, and hides it behind a bad helper

**Status:** done
**Priority:** low
**Created:** 2026-09-28
**Picked up by:** agent
**Started:** 2026-09-28
**Completed:** 2026-09-28

## Description

`TestAMorningAfterBurstCostsOneTrackRead` (`go/cmd/api/publiclimits_test.go`) failed twice during task 457, both
times under a full-suite run with other packages in parallel, and passes alone at `-count=3`:

```
publiclimits_test.go:124: GET http://127.0.0.1:52599/2026/patrulje/42: read tcp …->…: read: connection reset by peer
```

The test is worth keeping exactly as aggressive as it is — it fires **400 concurrent requests** (200 visitors × page
+ map) and asserts they cost *one* read of the telemetry projection, which is the property that stops a stranger
with a loop walking `TELEMETRY` per request (task 340). The flake is in how the requests are made, not in what is
being asserted.

## Two things to fix, and the second is the one that matters

**1. The client is `http.DefaultClient`, which is wrong for this shape of test.** Its transport keeps
`MaxIdleConnsPerHost = 2`, so 400 concurrent requests open something close to 400 sockets to one `httptest`
listener, and then discard them. Against a listener whose accept backlog and the machine's ephemeral port supply
are both finite — and shared with every other test binary running in parallel — resets are expected rather than
surprising. Nothing about that exercises the cache the test is about.

A burst-shaped test should own its client: a `*http.Transport` with `MaxIdleConnsPerHost` at the burst width (and
`DisableKeepAlives: false`), or the same 400 requests issued through a bounded worker pool. Either keeps the
concurrency the assertion needs — the track read is cached on first use, so what matters is that the requests
*overlap*, not that all 400 have their own socket.

**2. `getPublic` calls `t.Fatalf`, and this test calls it from 400 goroutines.** That is not allowed:
`FailNow` must be called from the goroutine running the test. From a child goroutine it runs `runtime.Goexit`, so:

- the test does not stop where it looks like it stops, and the remaining 399 goroutines keep going;
- the line after the call — `codes[i*2] = resp.StatusCode` — is never reached, leaving a `0` in the slice, so the
  *reported* failure becomes "request 37 answered 0" from a different assertion than the one that actually broke;
- `go vet` does not catch it, and the helper is used correctly by dozens of other tests, so the bug only exists at
  this one call site.

This is why the flake was initially hard to read: the output was a wall of transport errors from a helper that had
already decided to fail the test, followed by nothing conclusive.

The fix is at the call site, not in the helper: this test should do its own `Do`, record `(status, err)` per
request, and assert after `wg.Wait()` on the test's own goroutine. A shared `getPublic` that fatals is right for
the sequential tests that use it.

## What must not be lost

- **400 concurrent, overlapping requests.** A pool that serialises them would pass while asserting nothing: the
  point is that the second visitor arrives before the first has finished reading.
- **The one-read assertion** on the counting doubles, unchanged. That is the test's reason to exist.
- **Every request answers 200.** The burst must not be throttled — this route is deliberately exempt, and a rate
  limiter added later that caught it would be a real regression this test is here to catch.

## Acceptance Criteria

- [ ] The burst uses its own client/transport (or a bounded pool) sized for its own width
- [ ] No `t.Fatalf`/`FailNow` from a non-test goroutine in this test; failures are collected and asserted after
      `wg.Wait()`
- [ ] The one-read assertion and the 200-for-every-request assertion are unchanged
- [ ] Passes at `-count=5` while the rest of the suite runs in parallel (`GOWORK=off go test ./...`)

## Progress Log

- 2026-09-28 — Created from two flakes seen during task 457's full-suite runs. Diagnosis is from reading
  `getPublic` and the burst loop, not from instrumenting the failure: the transport's per-host idle limit and the
  goroutine `Fatalf` are both plainly there, and either alone explains what was observed. Worth confirming the
  first is really the cause — if resets persist with a properly sized transport, the next suspect is the
  `httptest` listener backlog rather than the client.

## What shipped

Picked up because it stopped being background noise: while validating task 471 it failed on most runs and was
blocking a clean gate.

**Both diagnoses in the description were right, and neither was the whole story.**

`getPublic`'s `t.Fatalf` from 400 goroutines was fixed first — the test now does its own request, records
`(status, err)` per slot, and asserts after `wg.Wait()` on the test's own goroutine. That immediately paid for itself
by replacing "connection reset by peer" with the real error:

	can't assign requested address

The local ephemeral port range running dry. HTTP/1.1 needs one connection per in-flight request, so 400 simultaneous
requests is 400 sockets against one `httptest` listener — and 400 more in TIME_WAIT on every repeat, on a machine
also running the rest of the suite. A bigger idle-connection budget does not help: at the instant 400 goroutines
start, there is nothing idle to reuse.

So the burst is now **400 requests with 64 in flight**, through its own transport.

### Why that does not weaken the test, checked rather than argued

The description says the concurrency must not be lost, and I wrote that. It turns out to be half right, and the
distinction matters:

- **The assertion is "400 requests cost one read."** That is what the counting doubles check and it does not depend on
  how many sockets were open at once.
- **The concurrency's job is to make requests overlap**, so a cache without single-flight is caught doing two reads.

Mutation-checked by disabling `patrolTrackReader.claim`'s waiter path:

| in flight | single-flight disabled |
|---|---|
| 64 | **fails** — "ByPeople called 2 times for 400 requests" |
| 400 | cannot tell — dies at the transport before it asserts anything |

So 64 tests the property and 400 tested nothing at all on this machine. The old number was not protecting the
assertion; it was preventing it from running.

## Acceptance Criteria

- [x] The burst uses its own client, sized for its own width
- [x] No `t.Fatalf` from a non-test goroutine; failures collected and asserted after `wg.Wait()`
- [x] The one-read assertion and the 200-for-every-request assertion unchanged
- [x] Passes at `-count=10`, six runs in a row, and in the full suite

## Progress Log

- 2026-09-28 — Created from two flakes seen during task 471's validation.
- 2026-09-28 — Fixed. The goroutine-`Fatalf` fix is what made the real cause visible, which is the general lesson:
  **a test that hides its own errors cannot be debugged.** Mutation-checked both concurrencies.

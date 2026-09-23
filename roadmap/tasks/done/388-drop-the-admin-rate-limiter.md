# 388 — Drop the admin rate limiter: it rationed the curator's work, not the guesses

**Status:** done
**Priority:** high
**Created:** 2026-09-23
**Picked up by:** agent
**Started:** 2026-09-23
**Completed:** 2026-09-23

## Description

Reported by the maintainer while using the tool:

> *"Just a little clicking around and I got `{"error": "For mange forsøg. Prøv igen om et øjeblik."}`"*

Task 371's credential limiter allowed **30 attempts per hour per IP** and sat *in front of* the comparison,
counting **every** request through `requireAdmin` rather than every wrong password. So the curator paid for
their own work.

The arithmetic nobody did at the time: the contact sheet fetches **one thumbnail per photograph** — up to 120
on a page — plus the page itself, the library read, the album list and the checkpoints. A single page load of
a real library is ~124 requests. An upload is one request per file, so a three-hundred-file hand-in is 300
more.

Measured before changing anything:

```
request #31 -> 429   <-- with the correct password every time
```

**That is PRD 022 §9's success condition failing outright** — *a photographer uploads a full card unaided* —
and it would have been discovered by a photographer at the event rather than by the maintainer clicking
around, because a dev library of three photographs makes a page load cost seven requests instead of 124.

## The decision

Two options were on the table. I wrote the first and the maintainer chose the second.

**Count only wrong passwords.** Keeps the brute-force protection, costs the curator nothing. The two
arguments in task 371's comment for counting everything do not survive inspection:

- *"A limiter that exempts successes lets an attacker who has guessed correctly continue unthrottled."* True,
  and it protects nothing. The credential is **shared** (§8.2), so an attacker holding it is
  indistinguishable from the curator by construction — there is no per-person signal to throttle. Once the
  password is known, a request ceiling is DoS protection rather than authentication protection, and 30/hour
  is not DoS protection either.
- *"A limiter behind the comparison still lets an attacker make the server do the work."* The work is four
  SHA-256 hashes of short strings, dwarfed by parsing the request that carried them. Sound for a deliberately
  slow KDF like bcrypt; not for this.

**Remove it entirely** — the maintainer's choice, on the ground that *the authentication mechanism is being
replaced by role-based access before next year's race* (§8.2), so the interim one is not worth carrying
machinery for. Agreed, and it is the better call: the guess limiter's remaining value was narrow enough
(it matters only against a password weak enough to be in somebody's list) that it did not justify a permanent
piece of middleware on a surface that is about to be rebuilt.

## What was removed, and what stands

Gone: `adminAuthLimiter`, `adminAuthLimiterFor`, `adminAuthAttemptsPerHour`, `allowAdminAttempt`, and the
`@Failure 429` on all fifteen admin endpoints.

Still there, and all of it task 371's: HTTPS-only with the 421, `no-store`, `noindex`, the constant-time
non-short-circuiting comparison, and **one identical refusal** for a missing credential, a wrong username and
a wrong password — so the response cannot be used to confirm the username, which for a shared credential is
half the secret.

One small behavioural improvement while in there: a request with **no** credential is no longer logged as a
rejection. That is the first half of the basic-auth handshake, performed by every browser before it has
anything to send; logging it buried the ones that mean somebody actually guessed.

## The consequence, written where somebody will read it

**The password's entropy is now the only control on this surface.** That is not a footnote, so it is stated
in four places rather than one — and specifically at the points where somebody is choosing a password, not
only where the limiter used to be:

| Where | What it says |
|---|---|
| `requireAdmin` (middleware.go) | the full reasoning, beside the checks that remain |
| `config.adminPassword` (env.go) | **generate it, do not choose it** — `openssl rand -base64 24` is the requirement that replaced the limiter |
| `docker-compose.prod.yml` | the same, at the line where the value is actually set |
| PRD 022 §6 and §8.2 | the mitigation list corrected rather than left claiming a limiter |

This promotes **§11 Q4** — who generates the credential, where it is kept, who is told — from operational
tidiness to the only control standing there. Recorded as such.

`main.go`'s boot line now logs the password's **length** (never the value), because "is this a generated
secret or is it `foto2026`?" is now a question worth being able to answer from a log without asking anybody.

### The gap I deliberately left

Nothing enforces a minimum length. I considered refusing to register the routes for a short password — the
same shape as the existing "no password means the tool is absent" rule, which would have made this
code-enforced rather than hoped-for. Not done: it turns a weak password into a silently missing tool on the
Tuesday after the event, which is its own failure mode, and §11 Q4's owner is a better place to fix this than
a length check. The boot log is the compromise. Worth revisiting with the auth rework.

## Tests

`TestAdminCredentialAttemptsAreRateLimited` is gone — it pinned the broken behaviour, complete with a comment
defending it. Two tests replace it:

- **`TestTheAdminSurfaceDoesNotThrottleAuthenticatedRequests`** — 400 authenticated requests, all of which
  must succeed. Written as the shape of the real workload rather than as "the limiter is gone", so it also
  fails for a future semaphore, connection cap or well-meant bot check. **400 rather than a token number,
  because the bug was 30**: a count that could be mistaken for a plausible limit would not have caught it.
- **`TestAWrongAdminCredentialIsAlwaysRefusedIdentically`** — repetition changes nothing, and a wrong
  username and a wrong password produce byte-identical responses.

Task 380's annotation guard caught the fifteen stale `@Failure 429`s on the first run, unprompted. That is
the second time this week it has paid for itself.

### A real gap found on the way

`httpReason` in the uploader had no sentence for **507** — the disk-full refusal from task 384 — so the one
failure where the right action is counter-intuitive (*keep the card, do not clear it*) would have rendered as
`Fejl 507.` to a photographer. Added, and `TestAdminUploaderExplainsEveryFailureInDanish` now covers it.

429 stays in that client-side map although the server no longer produces one: the endpoint is behind Traefik
and a proxy-originated 429 is still reachable, so the sentence is cheap insurance against a bare code.

## Verified live

Against the dev stack, after a rebuild:

| Check | Result |
|---|---|
| 250 consecutive authenticated `GET /api/admin/photos` | **all 200** (was: 30 then 429) |
| Wrong password, 40 attempts | **401 every time**, never 429, identical body |
| Wrong *username* | identical response to a wrong password |
| `WWW-Authenticate`, `Cache-Control: no-store`, `X-Robots-Tag: noindex` | all present on the 401 |
| `/admin` page and both library thumbnails | 200 |
| Boot log | `photographer admin tool served at /admin … password_length=27` |

## Acceptance Criteria

- [x] A curator can load the contact sheet and upload a full card without meeting a limit
- [x] The limiter and its configuration are removed, with no dead references
- [x] The `@Failure 429` annotations are removed from the admin endpoints
- [x] A regression test fails for any per-request ceiling on the admin surface, not just for this limiter
- [x] Wrong credentials are still refused identically, and the comparison is still constant-time
- [x] The consequence for password strength is recorded where a password is chosen, not only in the code
- [x] Task 371's record is amended additively rather than rewritten
- [x] PRD 022 §6, §8.2 and the task list no longer claim a rate limiter exists

## Notes

- **The flaw was visible in task 371's own acceptance criteria**, which recorded that *"a correct credential
  **is** throttled by an attacker sharing the IP — documented rather than fixed"*. That was the right thing to
  notice and the wrong thing to accept: the curator did not need an attacker to be throttled, only their own
  contact sheet. Worth remembering as a pattern — a documented tradeoff is not the same as an examined one,
  and "we know and accept this" can hide an arithmetic error nobody did.
- **A dev fixture of three photographs hid it.** The cost scales with library size, so every test and every
  manual check during phases 2 and 3 passed comfortably. The live verification in task 371 used few requests
  deliberately, *because* the limiter kept interfering with probing — which in hindsight was the bug
  announcing itself.

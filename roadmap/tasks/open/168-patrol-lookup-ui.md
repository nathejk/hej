# 168 — Patrol lookup UI

**Status:** open
**Priority:** medium
**Created:** 2026-08-31

## Description

The crew-only **"Slå patrulje op"** surface (PRD 007 §7), driving task 157's endpoint.

Deliberate design constraints — each one is what keeps this from becoming the browsable
index of minors' faces the PRD exists to avoid:

- A **distinct, secondary entry**, not merged into the main search field, so it is
  obvious that a patrol is a different thing asked for in a different way.
- **Numeric input, exact match.** No prefix search "for convenience", no patrol picker.
- Results in a **transient panel**, not appended to the directory listing.
- **No recent-lookups list.** It would accumulate into exactly the index we declined to
  build.
- **Nothing persisted.** Results live in memory for the duration of the view; no store
  write, no cache entry.
- Crew-only, and the entry point is not rendered for other roles — though the endpoint is
  what enforces it.

Offline behaviour is a first-class state, not an error: **"kræver forbindelse"** with a
pointer to the radio. The samarit's motivating scenario (03:00, woodland) is exactly
where this will fail, and the fallback is the radio and HQ — which is how it works today.
Do not show an empty patrol or a stale one.

Rows show face, name, **status** and **phone number** (task 157).

## Acceptance Criteria

- [ ] Crew-only entry point, visually distinct from the directory search.
- [ ] Exact numeric input; no partial matching or suggestions.
- [ ] Results transient — leaving the panel discards them; nothing in storage or caches.
- [ ] No recent-lookups history anywhere in the UI.
- [ ] Offline shows "kræver forbindelse" and points at the radio.
- [ ] A miss shows one neutral "ingen patrulje med det nummer".
- [ ] Rows show face, name, status, phone; withdrawn members marked.
- [ ] A comment in the component records *why* there is no history and no prefix search.

## Progress Log

- 2026-08-31 — Task created from PRD 007 §6 / §7.

# 317 — GlimtComposer — capture, caption, audience choice

**Status:** open
**Priority:** high
**Created:** 2026-09-17
**Picked up by:**
**Started:**
**Completed:**

## Description

PRD 019 §7. A shadcn `drawer`, not a route, so the feed stays behind it. Order: media strip →
caption → audience → share.

**Capture** reuses the portrait path: `getUserMedia` with the `<input type="file"
accept="image/*" capture>` fallback and the secure-context probe already in
`vue/src/components/profile/PhotoCapture.vue` / `usePortraitCapture.ts`. **Multi-select from
the library must work** — that is how most glimt will actually be made. Client-side downscale
and re-encode (canvas) before upload, to survive mobile data in a field.

**Audience is the most important thing on the screen.** A three-option radio-style list with
one line of consequence each — never a dropdown, never a default someone taps past. Defaults
to `group` (the narrowest). The `public` option carries the plainest consequence line we can
write, something close to *"Alle kan se det, også folk uden for Nathejk"*, because with no
approval queue in front of it (PRD 019 §0) **this sentence is the gate**.

Two quiet lines above the share button:
- *"Andre er også med på billedet — spørg dem først."*
- that the glimt is signed with the patrulje, not their name — reassurance worth a line
- and the Team section's reach, disclosed rather than discovered: "Min gruppe" is not "only my
  gruppe" (PRD 019 §6)

Retention copy states the **real configured number** from `/api/config` (task 310), never a
hard-coded one.

## Acceptance Criteria

- [ ] `GlimtComposer.vue` as a drawer, with media strip, caption (280 cap), audience, share
- [ ] Multi-select and camera capture both work; items removable before posting
- [ ] Client-side downscale/re-encode before upload
- [ ] Audience is a visible three-option list defaulting to `group`, each with a consequence line
- [ ] Consent line, hold-attribution reassurance, and Team-section disclosure present
- [ ] Retention figure read from `/api/config`
- [ ] Posts via the outbox (task 314) so an offline post is not lost
- [ ] `npm run test:unit` and `npm run build` pass

## Progress Log

- 2026-09-17 00:00 — Task created from PRD 019.

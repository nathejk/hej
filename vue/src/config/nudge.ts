// Where and when the portrait nudge appears (PRD 005 §6, task 146).
//
// The rule lives here rather than inside the component so it can be unit-tested: the component
// needs a browser, the decision does not. Same split as `router/gates.ts`.
//
// # Why a nudge exists at all
//
// The portrait step in onboarding is skippable, and PRD 005 §11 is explicit that asking once is not
// enough: the members most likely to skip are exactly the ones who then stay unidentifiable, and
// the photo's purpose is identification at night, when a samarit cannot otherwise tell who they are
// talking to. So the ask has to come back — but a prompt that cannot be silenced is one people
// learn to ignore, which is why dismissal is per session and why a portrait ends it permanently.

/**
 * Routes the nudge stays off, and the reason for each. Kept as data with the reasoning attached,
 * because a bare list of route names invites someone to "tidy" an entry away.
 *
 * - `sos` — an emergency page. Nothing may compete for attention on it, and a member opening it is
 *   not in a position to take a selfie.
 * - `profile` — the real photo control is already on that page. A banner above it asking for a
 *   photo would be noise next to the affordance it is asking you to use.
 *
 * The map is excluded by a different mechanism: it is the one full-bleed route, and the nudge is
 * only ever rendered inside the shell's normal content flow. That is deliberate rather than
 * incidental — the map is an operational surface during the race, and it already carries the
 * location pre-prompt and the offline notice.
 */
export const PORTRAIT_NUDGE_EXCLUDED_ROUTES = ['sos', 'profile']

export interface PortraitNudgeContext {
  /** A portrait is on file. Ends the nudge permanently — no flag involved. */
  hasPhoto: boolean
  /** The profile has been fetched. Before that, `hasPhoto: false` means "we don't know yet". */
  profileLoaded: boolean
  /** Dismissed for this session. In memory on purpose: a restart asks again. */
  dismissed: boolean
  routeName: string | null
  /** Full-bleed routes (the map) render outside the shell's content flow. */
  fullBleed: boolean
}

/**
 * Whether to show the nudge right now.
 *
 * Note the `profileLoaded` guard. Without it the nudge flashes on every cold start before
 * `GET /api/me/profile` resolves, because `hasPhoto` defaults to `false` — i.e. it would appear
 * most reliably for the members who have *already* done what it asks, which is the fastest way to
 * teach everyone to dismiss it on sight.
 */
export function showPortraitNudge(ctx: PortraitNudgeContext): boolean {
  if (!ctx.profileLoaded) return false
  if (ctx.hasPhoto) return false
  if (ctx.dismissed) return false
  if (ctx.fullBleed) return false
  if (ctx.routeName && PORTRAIT_NUDGE_EXCLUDED_ROUTES.includes(ctx.routeName)) return false
  return true
}

// The seam the install/device gates are switched off at (PRD 005 §6, §10).
//
// This module exists so the router guard has exactly one thing to ask, rather than the
// guard growing knowledge of runtime config, query parameters and QA flags. **Task 139
// fills it in**: the runtime flag from `GET /api/config` (so the gate can be disabled
// during an event without a rollback) and the dev/QA override (query param or
// localStorage, non-prod only) both belong here.
//
// Until then it is honestly `true`: the gates are on, with no way to bypass them. The
// alternative — inlining a half-override in the guard now — is how two competing bypass
// mechanisms end up shipping, and the one nobody documented is the one that gets found
// during an event.

/**
 * Whether the device/install/onboarding gates should run at all.
 *
 * Deliberately synchronous: the guard consults it on every navigation, including the
 * first paint on a cold start, and an awaited answer would produce the redirect flash
 * PRD 005 §6 rules out.
 */
export function gatesEnabled(): boolean {
  return true
}

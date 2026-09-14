import { defineStore } from 'pinia'
import { fetchWrapper, HttpError } from '@/helpers'
import type { Role } from '@/config/roles'

// The caller's own details, as returned by GET /api/me/profile (task 094).
export interface ProfileDetails {
  name: string
  role: Role
  /** Patrulje or klan. Empty means "not applicable", not missing. */
  team: string
  /** Crew section. Normally exactly one of team/section is set. */
  section: string
  address: string
  postalCode: string
  city: string
  phone: string
  /**
   * The guardian's number, and the reason this is `string | null` rather than
   * `string`:
   *
   *   null — this population has no guardian number at all (bandit, crew, gøgler)
   *   ''   — one is expected but is not registered
   *
   * The profile page hides the row for the first and shows "Ikke registreret" for
   * the second. Do not normalise these to one value; the BFF goes out of its way
   * to keep them apart (see go/cmd/api/profile.go).
   *
   * Note the BFF also sends `''` when the registered number is really the member's **own**
   * number (PRD 015, task 229): such a record cannot serve as an emergency contact, so it is
   * projected out rather than shown. The client therefore never needs to compare the two, and
   * must not start — the rule lives server-side, where it cannot be bypassed.
   */
  phoneParent: string | null
  /**
   * Whether the contact number is settled and can no longer be changed from the app: true once
   * the member has started, because check-in established the number at the counter (PRD 015,
   * task 231).
   *
   * Server-derived, like `confirmationRequired`. The client must render the number read-only
   * rather than offering an edit the BFF will refuse.
   */
  contactSettled: boolean
}

interface ProfileResponse {
  name: string
  role: Role
  team: string
  section: string
  address: string
  postal_code: string
  city: string
  phone: string
  phone_parent: string | null
  has_photo: boolean
  // Added by PRD 005 (task 134). Optional here on purpose: until the BFF sends it, an
  // absent field must mean "the confirmation step does not apply" rather than
  // "confirmation is required" — otherwise deploying the frontend first would put every
  // user in front of a step whose endpoint does not exist yet.
  confirmation_required?: boolean
  verified_at?: string | null
  // Added by PRD 015 (task 231). Optional for the same reason as confirmation_required: an
  // absent field must mean "not settled", so a frontend deployed ahead of the BFF offers the
  // correction path rather than hiding it.
  contact_settled?: boolean
}

/**
 * What the BFF returns when a recall attempt is wrong (PRD 015, task 227).
 *
 * `attemptsRemaining` counts down 2, 1, 0. `checkClosed` is true on the third failure: the step is
 * over, the outcome has been recorded server-side, and the member should be let into the app.
 */
export interface ContactCheckFailure {
  message: string
  attemptsRemaining: number
  checkClosed: boolean
}

/**
 * Reads the structured failure off a 400 from /me/profile/confirm.
 *
 * Returns null for anything else — a 429, a 503, an older BFF that answers with prose only — so a
 * caller can fall back to a plain message instead of inventing an attempt count. Deliberately
 * tolerant: the *count* is the server's to know, and guessing it here would recreate the
 * client-side counter this design exists to avoid.
 */
export function contactCheckFailure(err: unknown): ContactCheckFailure | null {
  if (!(err instanceof HttpError) || err.status !== 400) return null
  const body = err.body
  if (!body || typeof body !== 'object') return null
  const record = body as Record<string, unknown>
  if (typeof record.check_closed !== 'boolean') return null
  return {
    message: typeof record.error === 'string' ? record.error : err.message,
    attemptsRemaining: typeof record.attempts_remaining === 'number' ? record.attempts_remaining : 0,
    checkClosed: record.check_closed,
  }
}

// Shared in-flight fetch, so concurrent `ensureLoaded()` callers await one request and all of
// them see the result. Module-level rather than state: it is a promise, not data, and putting a
// promise in a Pinia store means Vue wraps it in a reactive proxy.
let inFlight: Promise<void> | null = null

// profile.store owns the signed-in user's own details (PRD 003).
//
// Deliberately separate from session.store: that store owns *authentication* and is
// consulted by the router guard on every navigation, so it must stay cheap and must
// never depend on a request that is allowed to fail. These details are page content.
export const useProfileStore = defineStore('profile', {
  state: () => ({
    details: null as ProfileDetails | null,
    loading: false,
    loaded: false,
    error: '',
    // Whether a portrait is on file. Comes from GET /api/me/profile's `has_photo`, and
    // is set directly by a successful upload.
    hasPhoto: false,
    // Whether this user still has to confirm their profile (PRD 005). **Server-derived**
    // from "has verified" OR "has started the event" — the client must not reimplement
    // that rule, and must not persist the answer: a localStorage copy would let a
    // reinstall skip the step, or re-ask a member who already confirmed, possibly
    // mid-event (PRD 005 §11).
    confirmationRequired: false,
    verifiedAt: null as string | null,
    // Bumped on every upload and appended to the image URL as a cache-buster.
    //
    // Necessary because the URL (`/api/me/photo`) is stable while its contents are not:
    // the response carries `Cache-Control: private, max-age=3600`, so after replacing a
    // portrait the browser would keep showing the old face for an hour.
    photoVersion: 0,
  }),
  getters: {
    /**
     * Initials for the user-menu avatar, e.g. "Freja Mikkelsen" → "FM".
     *
     * Returns '' when there is no name yet, so the caller can fall back to an icon
     * rather than rendering an empty circle.
     */
    initials: (state): string => {
      const name = state.details?.name?.trim()
      if (!name) return ''
      const parts = name.split(/\s+/)
      const first = parts[0]?.[0] ?? ''
      // Last word, not second: middle names are common and "Anne Sofie Jensen"
      // should read AJ, not AS.
      const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? '') : ''
      return (first + last).toUpperCase()
    },

    /**
     * URL for the current portrait, or null when there is none.
     *
     * Versioned (see photoVersion) so a replacement is shown immediately rather than an
     * hour later.
     */
    photoUrl: (state): string | null =>
      state.hasPhoto ? `/api/me/photo?v=${state.photoVersion}` : null,
  },
  actions: {
    // fetch loads the details. Never throws: the page renders its error state
    // instead, and the user menu that also reads this store must not be taken down
    // by a failed request — sign-out has to keep working offline.
    async fetch() {
      this.loading = true
      try {
        const data = await fetchWrapper.get<ProfileResponse>('/api/me/profile')
        this.details = {
          name: data.name,
          role: data.role,
          team: data.team,
          section: data.section,
          address: data.address,
          postalCode: data.postal_code,
          city: data.city,
          phone: data.phone,
          phoneParent: data.phone_parent,
          contactSettled: data.contact_settled ?? false,
        }
        this.hasPhoto = data.has_photo
        this.confirmationRequired = data.confirmation_required ?? false
        this.verifiedAt = data.verified_at ?? null
        this.error = ''
        this.loaded = true
      } catch {
        this.error = 'Kunne ikke hente dine oplysninger.'
      } finally {
        this.loading = false
      }
    },

    // ensureLoaded fetches once. Used by the user menu, which mounts on every page
    // and must not re-request the same data on each navigation.
    //
    // **Awaits an in-flight fetch** rather than returning immediately from it. The earlier
    // `if (this.loaded || this.loading) return` meant a second caller was told "done" while
    // the data was still missing — harmless for the user menu, which renders reactively, but
    // not for the onboarding flow, which has to decide *which step comes next* from
    // `confirmation_required` and `has_photo`. Deciding that against an unloaded profile skips
    // the profile-confirmation step (task 144).
    async ensureLoaded() {
      if (this.loaded) return
      if (inFlight) {
        await inFlight
        return
      }
      inFlight = this.fetch().finally(() => {
        inFlight = null
      })
      await inFlight
    },

    // uploadPhoto sends a captured portrait to the BFF.
    //
    // Unlike fetch() this one **throws**: the caller renders a retry affordance, and
    // swallowing the failure would leave the user believing there is a photo on file.
    // The BFF answers 503 for a retryable failure (broker down) and 400 for bytes it
    // could not read; both surface here.
    //
    // On success the portrait version is bumped rather than the image being re-fetched
    // by URL alone — see photoVersion.
    async uploadPhoto(blob: Blob) {
      const form = new FormData()
      form.append('photo', blob, 'portrait.jpg')
      await fetchWrapper.putForm('/api/me/photo', form)
      this.hasPhoto = true
      this.photoVersion += 1
    },

    // confirm records that the member has looked at their guardian number and
    // acknowledged it (PRD 005). Throws, like uploadPhoto: the step shows what went wrong
    // rather than pretending the confirmation landed.
    //
    // The two digits are checked **server-side** (task 135) so the acknowledgement is
    // recorded against a real answer. That is not a confidentiality measure: the full
    // number is already in this store, straight from GET /api/me/profile, exactly as
    // PRD 003 shipped it. The masking is there to make the member *look* at the number,
    // not to keep it from them (PRD 005 §11, 2026-08-30).
    async confirm(digits: string) {
      await fetchWrapper.post('/api/me/profile/confirm', {
        digits,
        acknowledged: true,
      })
      this.confirmationRequired = false
      this.verifiedAt = new Date().toISOString()
    },

    // setGuardian records a guardian number the member supplied themselves, plus their
    // acknowledgement that it can be reached (PRD 005, task 148).
    //
    // For the member who cannot recognise the number we hold. It does **not** overwrite the
    // registered number — `phoneParent` keeps coming from upstream — it records what the member
    // says can be reached.
    //
    // Refused with 409 once the member has started, because check-in established the number at the
    // counter (PRD 015, task 231). The step renders the number read-only in that state rather than
    // relying on this call to fail.
    //
    // Throws, like confirm(): the step shows what went wrong rather than pretending it landed.
    async setGuardian(phone: string) {
      await fetchWrapper.post('/api/me/profile/guardian', { phone, acknowledged: true })
      this.confirmationRequired = false
      this.verifiedAt = new Date().toISOString()
      // The register is unchanged, so `details.phoneParent` deliberately still shows what
      // Nathejk holds. Refetched so the page reflects whatever the server now derives.
      this.loaded = false
      await this.fetch()
    },

    // skipContactCheck records that the member gave up on the check (PRD 015, task 228).
    //
    // Called for the explicit "spring over", and after the third wrong answer — where the server
    // has already recorded the same outcome, so this is skipped (see the component).
    //
    // **Never throws.** Login is the only mandatory step (PRD 005 §6), so nothing here may stand
    // between the member and the app: no signal in a forest, or a broker outage, must not turn
    // giving up into a dead end. The outcome is then lost, which PRD 015 accepts because check-in
    // asks every member with no verified contact number anyway.
    //
    // 409 is a success: it means the check was already over for this session.
    async skipContactCheck() {
      try {
        await fetchWrapper.post('/api/me/profile/skip', {})
      } catch (err) {
        if (err instanceof HttpError && err.status === 409) return
        // Deliberately swallowed rather than surfaced. Nothing the member could do about it, and
        // nothing they need to know: they are being let through either way.
      }
    },

    // markPhotoState records whether a portrait exists. Exposed so a component that
    // learns the image failed to load (a blob gone missing) can correct the state
    // instead of showing a broken picture forever.
    markPhotoState(exists: boolean) {
      this.hasPhoto = exists
    },

    // clear drops the details on sign-out. Without this the next person to sign in
    // on a shared handset would see the previous user's name in the menu until the
    // first fetch resolved.
    clear() {
      this.details = null
      this.loaded = false
      this.error = ''
      this.hasPhoto = false
      this.photoVersion = 0
    },
  },
})

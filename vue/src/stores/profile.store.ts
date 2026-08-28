import { defineStore } from 'pinia'
import { fetchWrapper } from '@/helpers'
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
   */
  phoneParent: string | null
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
}

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
        }
        this.hasPhoto = data.has_photo
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
    async ensureLoaded() {
      if (this.loaded || this.loading) return
      await this.fetch()
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

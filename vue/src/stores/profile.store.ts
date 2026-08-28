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

    // clear drops the details on sign-out. Without this the next person to sign in
    // on a shared handset would see the previous user's name in the menu until the
    // first fetch resolved.
    clear() {
      this.details = null
      this.loaded = false
      this.error = ''
    },
  },
})

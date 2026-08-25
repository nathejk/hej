import { defineStore } from 'pinia'
import { fetchWrapper, HttpError } from '@/helpers'

// The app roles (mirrors the BFF's users.Role). Drives which pages the nav shows.
//
// These are *app* roles, not signup categories: the upstream data speaks in team
// types (patrulje/klan/crew/gøgler), and PRD 006's person projection owns the
// translation. Keep this list identical to `AllRoles` in
// `go/internal/users/directory.go` — they are one enum expressed twice.
//
// `crew` is the least-privileged fallback for a crew member whose function could
// not be determined from their section slug. It is not "crew with crew powers": an
// account lands there because classification failed.
export type Role =
  | 'spejder'
  | 'bandit'
  | 'postmandskab'
  | 'guide'
  | 'samarit'
  | 'gøgler'
  | 'crew'

export interface Identity {
  userId: string
  role: Role
}

interface IdentityResponse {
  user_id: string
  role: Role
}

// session.store owns authentication state. It is the single source of truth for
// "who is signed in", consumed by the router guards and the app shell/nav.
export const useSessionStore = defineStore('session', {
  state: () => ({
    user: null as Identity | null,
    // ready flips true once we've asked the BFF who we are at least once.
    ready: false,
  }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
    role: (state): Role | null => state.user?.role ?? null,
  },
  actions: {
    // fetchMe resolves the current session from the BFF. A 401 means "not
    // signed in" (a normal state), not an error.
    async fetchMe() {
      try {
        const data = await fetchWrapper.get<IdentityResponse>('/api/me')
        this.user = { userId: data.user_id, role: data.role }
      } catch (err) {
        if (err instanceof HttpError && err.status === 401) {
          this.user = null
        } else {
          throw err
        }
      } finally {
        this.ready = true
      }
    },

    // ensureReady lazily resolves the session once (used by the router guard).
    async ensureReady() {
      if (!this.ready) {
        await this.fetchMe()
      }
    },

    // requestPin asks the BFF to SMS a login PIN. The response is deliberately
    // identical whether or not the number is recognized (anti-enumeration).
    async requestPin(phone: string) {
      return fetchWrapper.post<{ message: string }>('/api/auth/request-pin', { phone })
    },

    // verify exchanges a phone + PIN for a session and stores the identity.
    async verify(phone: string, pin: string): Promise<Identity> {
      const data = await fetchWrapper.post<IdentityResponse>('/api/auth/verify', { phone, pin })
      this.user = { userId: data.user_id, role: data.role }
      this.ready = true
      return this.user
    },

    async logout() {
      try {
        await fetchWrapper.post('/api/auth/logout')
      } finally {
        this.user = null
      }
    },
  },
})

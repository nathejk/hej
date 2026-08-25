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

// One owner of a shared phone number, as offered by the chooser.
//
// Intentionally thin — the BFF sends a first name and a team and nothing else, because
// showing this list means showing one person something about the others on their
// number (task 079).
export interface ChoiceCandidate {
  user_id: string
  name: string
  /** Patrulje or klan, for a spejder or bandit. */
  team?: string
  /** Crew section. Exactly one of team/section is normally set. */
  section?: string
}

interface ChooseRequiredResponse {
  choice_token?: string
  candidates?: ChoiceCandidate[]
}

// session.store owns authentication state. It is the single source of truth for
// "who is signed in", consumed by the router guards and the app shell/nav.
export const useSessionStore = defineStore('session', {
  state: () => ({
    user: null as Identity | null,
    // ready flips true once we've asked the BFF who we are at least once.
    ready: false,
    // A pending disambiguation for a shared phone number. In memory only: the token
    // lives about a minute and bridges two requests, so persisting it would give it a
    // lifetime it should not have.
    choiceToken: null as string | null,
    choiceCandidates: [] as ChoiceCandidate[],
  }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
    role: (state): Role | null => state.user?.role ?? null,
    // needsChoice is true between a verified PIN and a chosen account.
    needsChoice: (state) => state.choiceToken !== null,
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
    //
    // Returns null when the number is shared by several people. That is a success,
    // not a failure: the PIN proved control of the phone, but not which of its owners
    // is holding it, so the BFF returns candidates plus a short-lived token instead of
    // a session (task 079). The caller shows the chooser and then calls choose().
    //
    // Branching on the presence of choice_token rather than on a status code is
    // deliberate — both outcomes are HTTP 200.
    async verify(phone: string, pin: string): Promise<Identity | null> {
      const data = await fetchWrapper.post<IdentityResponse & ChooseRequiredResponse>(
        '/api/auth/verify',
        { phone, pin },
      )

      if (data.choice_token) {
        this.choiceToken = data.choice_token
        this.choiceCandidates = data.candidates ?? []
        return null
      }

      this.clearChoice()
      this.user = { userId: data.user_id, role: data.role }
      this.ready = true
      return this.user
    },

    // choose completes login for a shared number by naming which owner you are.
    //
    // The token is held in memory only, never persisted: it is valid for about a
    // minute and exists to bridge two requests, so storing it would give it a lifetime
    // it should not have.
    async choose(userId: string): Promise<Identity> {
      if (!this.choiceToken) {
        // Reaching here means the UI offered a choice without a token — a bug, not
        // something a user can do.
        throw new Error('Ingen aktiv login-session. Bed om en ny kode.')
      }

      const data = await fetchWrapper.post<IdentityResponse>('/api/auth/choose', {
        token: this.choiceToken,
        user_id: userId,
      })

      this.clearChoice()
      this.user = { userId: data.user_id, role: data.role }
      this.ready = true
      return this.user
    },

    // clearChoice drops a pending disambiguation, e.g. when the user goes back to
    // re-enter their number.
    clearChoice() {
      this.choiceToken = null
      this.choiceCandidates = []
    },

    async logout() {
      try {
        await fetchWrapper.post('/api/auth/logout')
      } finally {
        this.user = null
        this.clearChoice()
      }
    },
  },
})

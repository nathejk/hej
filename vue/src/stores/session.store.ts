import { defineStore } from 'pinia'
import { fetchWrapper, HttpError, NetworkError } from '@/helpers'
import { clearIdentity, loadIdentity, saveIdentity } from '@/helpers/identity'
import { useAppStore } from '@/stores/app.store'
import type { Identity, Role } from '@/config/roles'

// The app roles and the signed-in identity live in @/config/roles, so that
// @/helpers/identity can validate a stored role without importing this store.
// Re-exported here because this store has always been their public home.
export type { Role, Identity } from '@/config/roles'
export { ALL_ROLES } from '@/config/roles'

interface IdentityResponse {
  user_id: string
  role: Role
  /**
   * How many profiles the caller's phone number carries (PRD 012).
   *
   * Drives whether "Skift profil" is offered at all. Absent or 0 means "one, or we could not
   * look" — both of which must hide the control, since offering it is what would mislead.
   */
  profile_count?: number
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
  /**
   * App role, used as the last discriminator when neither team nor section is set.
   *
   * Duplicate registrations often carry no affiliation at all, and then the role is the only
   * thing that differs between rows — see ProfileChooser.
   */
  role?: string
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
    // True when `user` came from the remembered identity rather than from the BFF,
    // i.e. we believe this is who you are but have not been able to confirm it
    // (task 090). Drives the offline notice; never used to grant anything.
    provisional: false,
    // A pending disambiguation for a shared phone number. In memory only: the token
    // lives about a minute and bridges two requests, so persisting it would give it a
    // lifetime it should not have.
    choiceToken: null as string | null,
    choiceCandidates: [] as ChoiceCandidate[],
    /**
     * How many profiles the signed-in number carries (PRD 012).
     *
     * 0 until `/api/me` answers. Deliberately **not** persisted with the remembered identity: on
     * an offline start we do not know it, and guessing would mean offering a switch that cannot
     * complete without the network anyway.
     */
    profileCount: 0,
  }),
  getters: {
    isAuthenticated: (state) => state.user !== null,
    role: (state): Role | null => state.user?.role ?? null,
    // needsChoice is true between a verified PIN and a chosen account.
    needsChoice: (state) => state.choiceToken !== null,
    /**
     * Whether this number has another profile to switch to.
     *
     * The client-side half of hiding the control from the majority who have one profile. The BFF
     * refuses a pointless switch regardless (409), so this is about not offering a dead end — not
     * about permission.
     */
    canSwitchProfile: (state) => state.profileCount > 1,
  },
  actions: {
    // fetchMe resolves the current session from the BFF. A 401 means "not
    // signed in" (a normal state), not an error.
    //
    // Three distinct outcomes, and the difference matters (task 090):
    //   * an identity      — confirmed, remembered for later offline starts
    //   * 401              — genuinely signed out; forget the remembered identity
    //   * no network       — unknown; fall back to the remembered identity and mark
    //                        it provisional. This is NOT a sign-out.
    // A network failure used to be rethrown, which rejected the router guard and
    // left a blank screen.
    async fetchMe() {
      const app = useAppStore()
      try {
        const data = await fetchWrapper.get<IdentityResponse>('/api/me')
        this.user = { userId: data.user_id, role: data.role }
        this.profileCount = data.profile_count ?? 0
        this.provisional = false
        saveIdentity(this.user)
        app.setOnline(true)
      } catch (err) {
        if (err instanceof NetworkError) {
          // The server said nothing, so nothing is known about the session. Keeping
          // the remembered identity is the honest reading of "unknown", and it is
          // safe: the cookie is what authorizes, and the BFF checks it per request.
          app.setOnline(false)
          const remembered = this.user ?? loadIdentity()
          if (remembered) {
            this.user = remembered
            this.provisional = true
          }
        } else if (err instanceof HttpError && err.status === 401) {
          this.user = null
          this.provisional = false
          clearIdentity()
          app.setOnline(true)
        } else {
          throw err
        }
      } finally {
        this.ready = true
      }
    },

    // ensureReady lazily resolves the session once (used by the router guard).
    //
    // Deliberately cannot reject. A rejected navigation guard aborts the navigation,
    // so no route component ever mounts and the user gets a white screen — exactly
    // the failure task 090 fixed. Whatever went wrong, it is better to enter the app
    // with whatever is known than to render nothing.
    async ensureReady() {
      if (this.ready) return
      try {
        await this.fetchMe()
      } catch {
        // A 5xx or a malformed response. `ready` is already true (fetchMe's finally),
        // so the guard proceeds on what is known: a remembered identity if there is
        // one, otherwise the login screen.
        const remembered = this.user ?? loadIdentity()
        if (remembered) {
          this.user = remembered
          this.provisional = true
        }
      }
    },

    // refresh re-asks the BFF who we are. Used when connectivity returns, to turn a
    // provisional identity back into a confirmed one — or to discover that the
    // session has in fact expired.
    async refresh() {
      this.ready = false
      await this.ensureReady()
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
      this.provisional = false
      this.ready = true
      saveIdentity(this.user)
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

      // The candidate count *is* the number of profiles on this number, so record it now rather
      // than waiting for the next /api/me — without this the switcher would be missing for the
      // rest of the session that just used the chooser.
      if (this.choiceCandidates.length > 0) {
        this.profileCount = this.choiceCandidates.length
      }

      this.clearChoice()
      this.user = { userId: data.user_id, role: data.role }
      this.provisional = false
      this.ready = true
      saveIdentity(this.user)
      return this.user
    },

    /**
     * Starts a profile switch for a number carrying several profiles (PRD 012).
     *
     * Fetches candidates plus a short-lived token from `/auth/switch` and stores them in the same
     * state the login chooser uses, so `choose()` completes either path. No SMS: the caller already
     * proved control of this number at login — see PRD 012 §8.
     *
     * **Throws**, unlike `fetchMe`. The caller has an explicit user action to report on, and
     * silently doing nothing after a tap is worse than a message. The current session is untouched
     * on failure: nothing is half-switched.
     */
    async startProfileSwitch(): Promise<ChoiceCandidate[]> {
      const data = await fetchWrapper.post<ChooseRequiredResponse>('/api/auth/switch')
      this.choiceToken = data.choice_token ?? null
      this.choiceCandidates = data.candidates ?? []
      return this.choiceCandidates
    },

    // clearChoice drops a pending disambiguation, e.g. when the user goes back to
    // re-enter their number.
    clearChoice() {
      this.choiceToken = null
      this.choiceCandidates = []
    },

    // logout ends the session. The remembered identity is dropped even if the request
    // fails: the user asked to be signed out, and leaving it behind would sign them
    // back in on the next offline start.
    async logout() {
      try {
        await fetchWrapper.post('/api/auth/logout')
      } finally {
        this.user = null
        this.provisional = false
        this.profileCount = 0
        clearIdentity()
        this.clearChoice()
      }
    },
  },
})

import type { Identity } from '@/config/roles'
import { ALL_ROLES } from '@/config/roles'
import type { Role } from '@/config/roles'

// Remembers the last identity the BFF confirmed, so the app can render offline.
//
// WHY THIS EXISTS (task 090). The session credential is an HttpOnly cookie: it
// survives offline and is sent with every request, but the app cannot read it. The
// only way to learn who the cookie belongs to is GET /api/me — which needs a
// network. With no signal the app therefore knew nothing about the user, bounced to
// /login, and offered a login that requires an SMS. In a forest at midnight that is
// the entire app gone.
//
// WHY THIS IS NOT A CREDENTIAL, and why localStorage is acceptable. What is stored
// is a user id and a role — the *answer* to "who am I", not the proof. The proof
// stays in the HttpOnly cookie, and the BFF authorizes every protected endpoint
// independently (the router guard has always been UX only). Editing this value by
// hand changes which tabs the nav draws and nothing else: the API still refuses.
// It is treated as provisional and replaced by the BFF's answer as soon as one
// arrives, and cleared on a real 401 or a sign-out.
//
// REJECTED: caching the /api/me response in the service worker. It reads more
// elegantly — one source of truth — but it makes the service worker responsible for
// authentication state, and a cached 200 would resurrect a session the server has
// already revoked, with the app unable to tell that the answer was stale. It also
// dies with the cache, which is evictable. A deliberate, inspectable local copy is
// the smaller mechanism.
const KEY = 'hej.identity'

interface StoredIdentity {
  userId: string
  role: string
}

function isRole(value: string): value is Role {
  return (ALL_ROLES as readonly string[]).includes(value)
}

// loadIdentity returns the remembered identity, or null if there is none or it is
// unusable. Anything unparseable is dropped rather than repaired: a bad value here
// would otherwise put the app in a state no sign-out could clear.
export function loadIdentity(): Identity | null {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return null

    const parsed = JSON.parse(raw) as Partial<StoredIdentity>
    if (typeof parsed?.userId !== 'string' || typeof parsed?.role !== 'string') {
      clearIdentity()
      return null
    }
    // The role list changes as the app grows (task 067 added three). A role this
    // build does not know would drive the nav off a cliff, so it is not trusted.
    if (!isRole(parsed.role)) {
      clearIdentity()
      return null
    }
    return { userId: parsed.userId, role: parsed.role }
  } catch {
    // Private-mode Safari can throw on access, and a corrupt value can throw on
    // parse. Neither is worth failing a page load over.
    return null
  }
}

export function saveIdentity(identity: Identity) {
  try {
    localStorage.setItem(KEY, JSON.stringify(identity))
  } catch {
    // Storage full or blocked: the app still works online, it just cannot render
    // offline. Not worth surfacing.
  }
}

export function clearIdentity() {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // See saveIdentity.
  }
}

import { defineStore } from 'pinia'

// Everything the app knows about its own installability (PRD 005 §8). Backs the install
// wall (task 119) and the escape hatch (task 121).
//
// Device-scoped only. Per-user state — whether this member has confirmed their profile —
// deliberately does not live here: it comes from the BFF, because localStorage would
// re-prompt a participant after a reinstall or a new phone, possibly mid-event
// (PRD 005 §11).

// `BeforeInstallPromptEvent` is not in the DOM lib — it is a Chromium extension to the
// spec that TypeScript does not ship a type for. Declared rather than cast to `any`, so
// the two things we actually rely on (`prompt()` and `userChoice`) are checked.
interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>
  readonly userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>
}

// The captured event, held outside the store.
//
// It is a live DOM event with a method that must be called on the original object, so it
// cannot go in Pinia state: making it reactive would have Vue wrap it in a proxy, and it
// would also show up in devtools serialisation. The store keeps a boolean mirror instead,
// which is all any consumer needs.
let pendingPrompt: BeforeInstallPromptEvent | null = null

const OVERRIDE_KEY = 'hej.install.continue-in-browser'

// Every localStorage access is wrapped: Safari throws on access in some privacy modes,
// and this value is read by the router guard on every navigation. An exception there
// would white-screen the app before anything is rendered — the same failure shape as
// task 090.
function readOverride(): boolean {
  try {
    return localStorage.getItem(OVERRIDE_KEY) === '1'
  } catch {
    return false
  }
}

function writeOverride(on: boolean) {
  try {
    if (on) localStorage.setItem(OVERRIDE_KEY, '1')
    else localStorage.removeItem(OVERRIDE_KEY)
  } catch {
    // Private mode can refuse. The override then lasts for this session only, which is
    // a degraded escape hatch rather than no escape hatch.
  }
}

/**
 * Registers the installability listeners. **Called from `main.ts` before `app.mount()`.**
 *
 * `beforeinstallprompt` fires once, and early. Register after mount and the event is
 * simply gone — one-tap install then degrades to manual add-to-home-screen instructions
 * on Chromium, the only platform where one tap is possible at all. Nothing errors when
 * that happens; the wall quietly shows the worse variant, and it will not reproduce
 * reliably in dev. Hence a plain function called at a specific point rather than an
 * `onMounted` hook in the wall.
 *
 * Pinia is installed before this runs, so the store is usable here.
 */
export function initInstallPrompt() {
  window.addEventListener('beforeinstallprompt', (event) => {
    // Suppress Chromium's own mini-infobar. Two competing install affordances on the
    // same screen is worse than either alone, and the wall is the one that can explain
    // *why* the app has to be installed.
    event.preventDefault()
    pendingPrompt = event as BeforeInstallPromptEvent
    useInstallStore().setCanPrompt(true)
  })

  window.addEventListener('appinstalled', () => {
    pendingPrompt = null
    const store = useInstallStore()
    store.setCanPrompt(false)
    store.markInstalled()
  })
}

export const useInstallStore = defineStore('install', {
  state: () => ({
    // A `beforeinstallprompt` event is held and has not been consumed.
    canPrompt: false,
    // Set from `appinstalled`. Note this only ever fires in the tab that did the
    // installing — it is not a way to answer "is this app installed?" in general, which
    // is why the wall also offers "jeg har allerede installeret appen".
    installed: false,
    // The persisted escape hatch. Read eagerly: the router guard needs it synchronously
    // on the very first navigation.
    continueInBrowser: readOverride(),
  }),
  actions: {
    setCanPrompt(can: boolean) {
      this.canPrompt = can
    },
    markInstalled() {
      this.installed = true
    },
    /**
     * Shows the native install prompt. Returns the user's choice, or `null` if there was
     * no prompt to show.
     *
     * The event is **single-use**: Chromium will not accept a second `prompt()` call on
     * the same event, and a fresh `beforeinstallprompt` only arrives on a later page
     * load. So the reference is dropped and `canPrompt` cleared whichever way the user
     * answered — otherwise the button stays enabled and does nothing, which reads as a
     * broken app rather than a declined prompt.
     */
    async promptInstall(): Promise<'accepted' | 'dismissed' | null> {
      const event = pendingPrompt
      if (!event) return null
      try {
        await event.prompt()
        const { outcome } = await event.userChoice
        return outcome
      } catch {
        // A prompt that throws (already consumed, or the gesture requirement not met)
        // must still leave the UI honest, hence the finally block below.
        return null
      } finally {
        pendingPrompt = null
        this.canPrompt = false
      }
    },
    setContinueInBrowser(on: boolean) {
      this.continueInBrowser = on
      writeOverride(on)
    },
  },
})

<script setup lang="ts">
// The user menu in the trailing corner of the app bar (PRD 003 §7, decided
// 2026-08-28): an avatar button opening a menu with who you are, a link to
// Min profil, and Log ud.
//
// This is the app's ONLY sign-out action, and PRD 005 owns where it goes. It used
// to be a bare "Log ud" button in App.vue; moving it here keeps profile out of the
// bottom nav's five visible slots and puts sign-out somewhere users already look
// for it.
//
// Not present on full-bleed routes (`/maps`), which have no app bar. That is
// accepted rather than worked around — the map's trailing corner is the layer
// switcher, the bottom nav is always there to leave the map, and neither profile
// nor sign-out is an in-map action.
//
// Built on the shadcn-vue dropdown-menu primitive rather than a hand-rolled popover,
// per .rules: focus trapping, escape/outside-press dismissal and roving-focus
// keyboard navigation are the whole reason to use it.
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { CircleUser, LogOut, User, Users } from '@lucide/vue'
import { useSessionStore } from '@/stores/session.store'
import { useProfileStore } from '@/stores/profile.store'
import { ROLE_LABELS } from '@/config/roles'
import { NetworkError } from '@/helpers'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import ProfileChooser from '@/components/auth/ProfileChooser.vue'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

const router = useRouter()
const session = useSessionStore()
const profile = useProfileStore()

// The name comes from the profile endpoint, not from the session: the signed-in
// identity is only `{userId, role}` (deliberately — it is what the router guard
// needs). Fetched once here because this component mounts on every non-full-bleed
// page, which also means the profile page usually finds the data already loaded.
//
// Everything below has to render before this resolves, and while it is failing:
// sign-out must keep working offline.
onMounted(() => void profile.ensureLoaded())

async function signOut() {
  await session.logout()
  // Drop the cached details, or the next person to sign in on a shared handset sees
  // the previous user's name in this menu until the first request comes back.
  profile.clear()
  await router.replace({ name: 'welcome' })
}

// —— Skift profil (PRD 012) ——
//
// For a phone number carrying several profiles, which in the real data is usually one person with
// duplicate registrations rather than a parent juggling siblings (PRD 006 §11 Q1).

const switching = ref(false)
const switchBusy = ref(false)
const switchError = ref('')

async function openSwitcher() {
  switchError.value = ''
  switchBusy.value = true
  switching.value = true
  try {
    await session.startProfileSwitch()
  } catch (err) {
    // The switch needs the BFF, so this is the one place the feature cannot degrade. Say which
    // kind of failure it was: "no connection" is actionable, "something went wrong" is not.
    switchError.value =
      err instanceof NetworkError
        ? 'Kræver forbindelse. Find et sted med signal og prøv igen.'
        : 'Kunne ikke hente dine profiler. Prøv igen.'
  } finally {
    switchBusy.value = false
  }
}

async function switchTo(userId: string) {
  switchError.value = ''
  switchBusy.value = true
  try {
    await session.choose(userId)
  } catch {
    // The current session is untouched on failure — nothing is half-switched — so the user stays
    // signed in as they were and can simply close the dialog.
    switchError.value = 'Kunne ikke skifte profil. Prøv igen.'
    switchBusy.value = false
    return
  }

  // A **full page load**, not a router push. Every store in memory belongs to the profile that just
  // stopped being signed in — the contacts directory, favourites, the profile details — and their
  // persisted copies are already keyed per profile (task 180). Reloading is the one move that
  // cannot leave a stale one behind through a path somebody forgets to reset, and on a PWA the
  // assets come from cache so it costs little.
  //
  // Landing on the map rather than staying put: the current route may be role-gated and no longer
  // permitted for the new profile, and the guard bouncing the user elsewhere would read as a glitch.
  window.location.assign('/maps')
}
</script>

<template>
  <DropdownMenu>
    <!-- min-h/min-w rather than a padded icon: the tap target is a hard 44px
         requirement (task 010), and the avatar itself is 32px. -->
    <DropdownMenuTrigger
      class="flex min-h-11 min-w-11 items-center justify-center rounded-full text-slate-500 focus-visible:ring-2 focus-visible:ring-slate-400 focus-visible:outline-none"
      aria-label="Din profil og konto"
    >
      <Avatar>
        <!-- The portrait once there is one; initials until then, which doubles as a
             standing nudge to take one. -->
        <AvatarImage v-if="profile.photoUrl" :src="profile.photoUrl" alt="Dit billede" />
        <AvatarFallback>
          <template v-if="profile.initials">{{ profile.initials }}</template>
          <User v-else class="h-4 w-4" aria-hidden="true" />
        </AvatarFallback>
      </Avatar>
    </DropdownMenuTrigger>

    <!-- The primitive defaults to the trigger's width, which for a 44px avatar would
         be unreadable; align to the trailing edge so it cannot overflow the screen. -->
    <DropdownMenuContent align="end" class="w-56">
      <!-- Answers "who am I signed in as?" without navigating. Not a menu item: it is
           not actionable, and making it focusable would put a dead stop in the
           keyboard order. -->
      <div class="px-2 py-1.5">
        <p class="truncate text-sm font-medium text-slate-900">
          {{ profile.details?.name || 'Min profil' }}
        </p>
        <p v-if="session.role" class="truncate text-xs text-slate-500">
          {{ ROLE_LABELS[session.role] }}
          <template v-if="profile.details?.team"> · {{ profile.details.team }}</template>
          <template v-else-if="profile.details?.section">
            · {{ profile.details.section }}
          </template>
        </p>
      </div>

      <DropdownMenuSeparator />

      <DropdownMenuItem @select="router.push({ name: 'profile' })">
        <CircleUser class="h-4 w-4" aria-hidden="true" />
        Min profil
      </DropdownMenuItem>

      <!-- Only when this number actually carries another profile. A control answering "you have
           nothing to switch to" is worse than no control, and the large majority have one profile
           (PRD 012 §6). The BFF refuses a pointless switch regardless. -->
      <DropdownMenuItem v-if="session.canSwitchProfile" @select="openSwitcher">
        <Users class="h-4 w-4" aria-hidden="true" />
        Skift profil
      </DropdownMenuItem>

      <DropdownMenuSeparator />

      <DropdownMenuItem variant="destructive" @select="signOut">
        <LogOut class="h-4 w-4" aria-hidden="true" />
        Log ud
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>

  <!-- Outside the menu: the dropdown closes on select, and a dialog nested inside it would be torn
       down with it. -->
  <Dialog v-model:open="switching">
    <DialogContent>
      <DialogHeader>
        <DialogTitle>Skift profil</DialogTitle>
        <DialogDescription>
          Dette nummer har flere profiler. Hvem vil du bruge appen som?
        </DialogDescription>
      </DialogHeader>

      <p v-if="switchError" class="text-sm text-red-600">{{ switchError }}</p>

      <p v-else-if="switchBusy && session.choiceCandidates.length === 0" class="text-sm text-slate-500">
        Henter dine profiler …
      </p>

      <!-- The same list the login chooser uses (task 181), so a person is identified the same way
           on both surfaces. -->
      <ProfileChooser
        v-else
        :candidates="session.choiceCandidates"
        :busy="switchBusy"
        @choose="switchTo"
      />
    </DialogContent>
  </Dialog>
</template>

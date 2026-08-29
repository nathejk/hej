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
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { CircleUser, LogOut, User } from '@lucide/vue'
import { useSessionStore } from '@/stores/session.store'
import { useProfileStore } from '@/stores/profile.store'
import { ROLE_LABELS } from '@/config/roles'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
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

      <DropdownMenuSeparator />

      <DropdownMenuItem variant="destructive" @select="signOut">
        <LogOut class="h-4 w-4" aria-hidden="true" />
        Log ud
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>

<script setup lang="ts">
// One row of the contacts directory (PRD 007 §7, task 164).
//
// Layout is fixed by the maintainer's direction (2026-08-31): avatar left, member name with
// the team/group name in smaller grey print below it, phone number on the right. Tapping the
// row — not the number — opens the person's profile.
//
// # Two tap targets, deliberately separated
//
// The row opens a profile; the number places a call. Those are very different outcomes, so the
// number is its own link with its own hit area, and the favourite toggle sits on the other side
// of the row from it. Nobody should start a call while reaching for a star, in a forest, in
// gloves, at 03:00.
//
// # No portrait is a normal state
//
// Portraits are skippable at onboarding, so many are simply absent. The fallback shows initials
// — visibly "no photo" rather than a broken image (task 169).
//
// Composes the shadcn-vue `avatar` primitive directly, like PRD 003's ProfilePhoto does. It is
// deliberately not reused from there: that component owns an upload flow, which a read-only
// directory row has no business inheriting.
import { computed, ref } from 'vue'
import { ImageOff, Phone, Star } from '@lucide/vue'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { formatPhone } from '@/helpers'
import type { ContactEntry } from '@/stores/contacts.store'

const props = defineProps<{
  entry: ContactEntry
  favourite?: boolean
}>()

const emit = defineEmits<{
  open: [id: string]
  toggleFavourite: [id: string]
}>()

// The portrait URL carries the version, so a replaced portrait busts the browser cache while an
// unchanged one is never refetched. The BFF serves these content-addressed with a long
// `private` max-age, which is what makes the pane work offline without a hand-rolled image
// cache.
const photoUrl = computed(() =>
  props.entry.portraitVersion
    ? `/api/contacts/people/${encodeURIComponent(props.entry.id)}/photo?size=thumb&v=${props.entry.portraitVersion}`
    : undefined,
)

// Initials from the first and last name parts: "Freja Mikkelsen" → "FM". Danish letters survive
// because this slices characters rather than matching an ASCII range.
const initials = computed(() => {
  const parts = props.entry.name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase()
  return (parts[0].slice(0, 1) + parts[parts.length - 1].slice(0, 1)).toUpperCase()
})

// The secondary line: the group they are in, plus their crew function when there is one. Both
// are display-only context, and either may be absent.
const subtitle = computed(() => {
  const group = props.entry.groups[props.entry.groups.length - 1]?.label
  const parts = [props.entry.crewFunction, group].filter(Boolean)
  return parts.join(' · ')
})

// Whether to offer a call.
//
// The number's *presence* is the answer, because the BFF has already made that decision: it sends
// a number unless the member is `released` — the one status where they have left the area and been
// handed to a guardian (2026-09-01). Everyone else is in or around the event and worth reaching,
// including a member who is out of the race but back with their patrol.
//
// Deliberately no longer gated on `stillInRace`. That conflated two questions and suppressed the
// number for every withdrawn member, including people sitting in a car on the way to HQ — exactly
// the people a samarit needs to ring. The row renders what it is given rather than re-deciding it.
const callable = computed(() => Boolean(props.entry.phone))

// Whether the thumbnail actually loaded. reka-ui's Avatar swaps in the fallback by itself when an
// image fails, so names-only degradation is free — but the fallback alone cannot say *why* there
// is no face, and the two reasons need different reactions from the reader:
//
//   - no portrait on file (skippable at onboarding, so common) — nothing to do about it;
//   - a portrait exists but this device has not got the bytes (never synced, or the OS evicted
//     the cache, which iOS does within days for an unused web app) — worth knowing, because it
//     means going online will fill it in.
//
// PRD 007 §6 asks for exactly this distinction: "no photo" must not read as "not synced yet".
const imageFailed = ref(false)

function onImageStatus(status: 'idle' | 'loading' | 'loaded' | 'error') {
  imageFailed.value = status === 'error'
}

// A portrait we know exists but cannot show.
const portraitMissingLocally = computed(() => Boolean(props.entry.portraitVersion) && imageFailed.value)
</script>

<template>
  <div
    class="flex items-center gap-3 border-b border-slate-800/60 px-4 py-2.5 last:border-b-0"
    :class="{ 'opacity-70': !entry.stillInRace }"
  >
    <!-- Row body: the profile tap target. A button rather than a link, because the profile is
         opened by the parent view (which owns the router) — and a button gets keyboard and
         screen-reader semantics for free. -->
    <button
      type="button"
      class="flex min-w-0 flex-1 items-center gap-3 text-left"
      :aria-label="`Vis ${entry.name}`"
      @click="emit('open', entry.id)"
    >
      <Avatar size="lg" class="relative">
        <AvatarImage
          v-if="photoUrl"
          :src="photoUrl"
          :alt="entry.name"
          @loading-status-change="onImageStatus"
        />
        <AvatarFallback class="bg-slate-700 text-slate-200">{{ initials }}</AvatarFallback>
        <!-- Marks "we have a photo of this person, this device just does not have it" — as
             distinct from the initials alone, which mean there is no photo on file. Small and
             unobtrusive: it is a fact about the device, not about the person. -->
        <span
          v-if="portraitMissingLocally"
          class="absolute -bottom-0.5 -right-0.5 rounded-full bg-slate-900 p-0.5"
          title="Billedet er ikke hentet til denne enhed"
          aria-label="Billedet er ikke hentet til denne enhed"
        >
          <ImageOff class="size-3 text-slate-400" />
        </span>
      </Avatar>

      <span class="min-w-0 flex-1">
        <span class="block truncate text-[15px] leading-tight text-slate-100">
          {{ entry.name }}
        </span>
        <!-- Smaller grey print below the name, per the agreed layout. -->
        <span v-if="subtitle" class="block truncate text-xs leading-tight text-slate-400">
          {{ subtitle }}
        </span>
        <!-- The status marking for a member who has left the race (task 160). Text, not only a
             colour or an opacity: those are invisible to anyone who cannot see the difference,
             and this is the fact that explains a missing phone number. -->
        <span v-if="!entry.stillInRace" class="block text-xs leading-tight text-amber-400">
          Ude af løbet
        </span>
      </span>
    </button>

    <!-- Favourite toggle, kept on the opposite side of the row from the number so a thumb
         aiming here cannot start a call. -->
    <button
      type="button"
      class="shrink-0 p-2 text-slate-500"
      :class="{ 'text-amber-300': favourite }"
      :aria-pressed="favourite ? 'true' : 'false'"
      :aria-label="
        favourite ? `Fjern ${entry.name} fra favoritter` : `Tilføj ${entry.name} til favoritter`
      "
      @click="emit('toggleFavourite', entry.id)"
    >
      <Star class="size-4" :fill="favourite ? 'currentColor' : 'none'" />
    </button>

    <!-- The number, right-aligned, as its own action. `tel:` is fine; there is deliberately no
         copy, share or export affordance anywhere in this pane. -->
    <a
      v-if="callable"
      :href="`tel:${entry.phone}`"
      class="flex shrink-0 items-center gap-1.5 text-sm tabular-nums text-slate-300"
      :aria-label="`Ring til ${entry.name}`"
    >
      <Phone class="size-3.5 text-slate-500" />
      <span class="hidden sm:inline">{{ formatPhone(entry.phone!) }}</span>
    </a>
  </div>
</template>

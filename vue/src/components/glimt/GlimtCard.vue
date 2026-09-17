<script setup lang="ts">
// One glimt in the feed (PRD 019 §7, task 316).
//
// # There is no avatar, and no name
//
// A glimt is attributed to its **hold**, never to the person who posted it (PRD 019 §0b, task 302).
// The payload carries no author at all, so there is nothing to render even if this component wanted
// to — which is the point of enforcing it in the response type rather than here. That also means
// there is **no avatar slot in this design and no portrait to fetch**, which is a real saving on a
// grid of a hundred cards.
//
// Your own card reads "Dit hold". Not your name: it would be the one payload most likely to be
// cached on disk, and "Dit hold" plus a Slet action conveys ownership without it.
//
// # The attribution is a link, and that is the main way anyone browses
//
// Tapping it opens that hold's collection (task 326) — the post-race surface (PRD 019 §0a.1). A
// filter buried in a menu would not get used; a tappable line that is already on every card does.
//
// # Anmeld is one tap, on every card
//
// With no approval queue in front of the public scope (PRD 019 §0), reporting is the safety
// mechanism. It lives in the same overflow menu as everything else rather than behind a long-press,
// and `glimtActions()` decides what is offered so the rule is testable without mounting this.
import { computed } from 'vue'
import { CloudUpload, EllipsisVertical } from '@lucide/vue'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import GlimtMediaStrip from '@/components/glimt/GlimtMediaStrip.vue'
import {
  attributionLine,
  audienceLabel,
  audienceVariant,
  glimtActions,
  ownAttribution,
  relativeTime,
} from '@/components/glimt/glimtPresentation'
import type { GlimtAction } from '@/components/glimt/glimtPresentation'
import type { Glimt as GlimtEntry } from '@/stores/glimt.store'

const props = defineProps<{
  glimt: GlimtEntry
  /** Suppress the hold link — the hold collection does not need to link to itself. */
  hideHoldLink?: boolean
  /**
   * Override the overflow menu.
   *
   * The moderation view passes `moderationActions(glimt)` (task 309). A prop rather than a
   * `moderating` flag, so this component holds no opinion about who is looking at it — which is what
   * lets the queue reuse it instead of forking a second card that would drift from this one.
   */
  actions?: GlimtAction[]
}>()

const emit = defineEmits<{
  open: [ordinal: number]
  openHold: [number: string]
  action: [key: GlimtAction['key']]
}>()

const attribution = computed(() =>
  props.glimt.own ? ownAttribution(props.glimt.hold.group) : attributionLine(props.glimt.hold),
)

// Crew have no hold number, so there is no collection to link to — and neither has a glimt whose
// hold was never numbered. The line then renders as plain text rather than a link that 404s.
//
// A queued glimt has no attribution at all yet (the server freezes it at creation), so it never
// links either.
const holdLinkable = computed(
  () => !props.hideHoldLink && !props.glimt.pending && props.glimt.hold.number.length > 0,
)

const actions = computed(() => props.actions ?? glimtActions(props.glimt))
</script>

<template>
  <Card class="overflow-hidden py-0 gap-0">
    <header class="flex items-center gap-2 px-3 py-2">
      <div class="min-w-0 flex-1">
        <!-- The attribution, tappable. A button rather than a RouterLink because the parent owns
             navigation: the feed goes to a route, the moderation view does not navigate at all. -->
        <button
          v-if="holdLinkable"
          type="button"
          class="block max-w-full truncate text-left text-sm font-medium underline-offset-2 hover:underline"
          @click="emit('openHold', glimt.hold.number)"
        >
          {{ attribution }}
        </button>
        <p v-else class="truncate text-sm font-medium">{{ attribution }}</p>

        <p class="text-xs text-muted-foreground">
          {{ relativeTime(glimt.createdAt) }}
        </p>
        <!-- Extra context under the timestamp. Used by the moderation queue for the author, which
             is the one thing this card is otherwise forbidden from knowing (PRD 019 §0b) — so it
             arrives as markup from the caller rather than as a field on the glimt. -->
        <slot name="meta" />
      </div>

      <!-- Extra chips before the audience badge; the queue puts its report count here. -->
      <slot name="badges" />

      <Badge :variant="audienceVariant(glimt.audience)" class="shrink-0">
        {{ audienceLabel(glimt.audience) }}
      </Badge>

      <!-- Hidden is only ever true on a glimt the caller wrote or moderates — anyone else does not
           receive it — so saying so is safe, and an author is entitled to know their post was taken
           down rather than silently discovering nobody can see it. -->
      <Badge v-if="glimt.hidden" variant="destructive" class="shrink-0">Skjult</Badge>

      <!-- Still in the outbox (task 325). PRD 019 §5 requires the entry itself to say so, not just a
           counter above the feed — and the wording must never imply a background upload, because iOS
           does not run a backgrounded web app. "Venter" is the honest verb. -->
      <Badge v-if="glimt.pending" variant="secondary" class="shrink-0 gap-1">
        <CloudUpload class="size-3" aria-hidden="true" />
        Venter
      </Badge>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <!-- ≥44px touch target: this is the control that reports a photograph somebody objects
               to, used one-handed in the dark. -->
          <Button variant="ghost" size="icon" class="size-11 shrink-0" aria-label="Flere handlinger">
            <EllipsisVertical class="size-5" aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem
            v-for="action in actions"
            :key="action.key"
            :variant="action.destructive ? 'destructive' : 'default'"
            @select="emit('action', action.key)"
          >
            {{ action.label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </header>

    <GlimtMediaStrip :glimt="glimt" @open="(o) => emit('open', o)" />

    <!-- Real text, not an overlay on the image: a caption drawn onto a photograph is unreadable to
         a screen reader and often to a person. -->
    <CardContent v-if="glimt.caption" class="px-3 py-2">
      <p class="text-sm whitespace-pre-wrap break-words">{{ glimt.caption }}</p>
    </CardContent>
  </Card>
</template>

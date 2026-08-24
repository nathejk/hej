<script setup lang="ts">
import { ref } from 'vue'
import { Layers, Check } from '@lucide/vue'
import { baseLayers, type BaseLayerKey } from '@/config/map'

// Base-layer picker. Hand-rolled rather than shadcn's Popover/RadioGroup: this
// floats inside the map's control stack and needs to stay tap-friendly and
// compact, which the popover primitive's portal + centred positioning fights.
// Standard components are still the default elsewhere (see .rules).
const model = defineModel<BaseLayerKey>({ required: true })

const open = ref(false)
const keys = Object.keys(baseLayers) as BaseLayerKey[]

function choose(key: BaseLayerKey) {
  model.value = key
  open.value = false
}
</script>

<template>
  <div class="relative">
    <button
      type="button"
      class="flex h-11 w-11 items-center justify-center rounded-xl bg-white/95 text-slate-700 shadow-md ring-1 ring-slate-900/10"
      :aria-expanded="open"
      aria-label="Vælg kortlag"
      @click="open = !open"
    >
      <Layers class="h-5 w-5" aria-hidden="true" />
    </button>

    <div
      v-if="open"
      class="absolute right-0 top-12 w-56 overflow-hidden rounded-xl bg-white/95 shadow-lg ring-1 ring-slate-900/10"
      role="radiogroup"
      aria-label="Kortlag"
    >
      <button
        v-for="key in keys"
        :key="key"
        type="button"
        class="flex min-h-[3rem] w-full items-center gap-2 px-3 py-2 text-left text-sm text-slate-700"
        role="radio"
        :aria-checked="model === key"
        @click="choose(key)"
      >
        <Check
          class="h-4 w-4 shrink-0"
          :class="model === key ? 'text-slate-900' : 'invisible'"
          aria-hidden="true"
        />
        <span class="flex-1">
          {{ baseLayers[key].label }}
          <span v-if="baseLayers[key].note" class="block text-xs text-slate-500">
            {{ baseLayers[key].note }}
          </span>
        </span>
      </button>
    </div>
  </div>
</template>

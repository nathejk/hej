import type { VariantProps } from 'class-variance-authority'
import { cva } from 'class-variance-authority'

export { default as Badge } from './Badge.vue'

// Generated to match the neighbouring primitives (alert, button) rather than hand-rolled, per
// `.rules`: a standard shadcn-vue component exists for this, and generated primitives are owned
// source we edit in place instead of wrapping.
//
// LOCAL NOTE: the `warning` and `success` variants are not upstream. `warning` exists because the
// readiness view (PRD 009, task 187) has a genuinely three-way state to show — present, old, gone — and
// rendering "old" as `destructive` would tell a participant something is wrong when their map is merely a
// week stale. `success` (task 266) is the green counterpart the drawer's on-time verdict needs: "på tid"
// must read as reassurance, and neither `default` (brand) nor `secondary` (neutral) carries that. Both are
// kept as variants rather than one-off classes so the next surface with the same distinction reuses the
// colour instead of inventing a new one.
export const badgeVariants = cva(
  'inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 [&>svg]:pointer-events-none [&>svg]:size-3',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-transparent bg-secondary text-secondary-foreground',
        outline: 'border-border text-foreground',
        warning: 'border-transparent bg-amber-100 text-amber-900 dark:bg-amber-950 dark:text-amber-200',
        success: 'border-transparent bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200',
        destructive: 'border-transparent bg-destructive/10 text-destructive',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export type BadgeVariants = VariantProps<typeof badgeVariants>

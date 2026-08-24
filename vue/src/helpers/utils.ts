import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// cn merges conditional class lists and resolves Tailwind conflicts (the later
// utility wins), which is what shadcn-vue's generated components expect.
//
// Lives in `helpers/` rather than the shadcn CLI's default `lib/utils` because
// shared utilities live in `@/helpers` in this repo — `components.json` points
// the CLI here, so generated components import from `@/helpers/utils`.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

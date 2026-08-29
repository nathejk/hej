import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vitest/config'

// Vitest is configured separately from vite.config.ts on purpose: that config loads the
// PWA plugin and the Tailwind plugin, neither of which a unit test needs, and both of
// which cost time and can fail for reasons unrelated to the code under test.
//
// `environment: 'node'` is enough — the modules under test take their browser
// environment as an argument rather than reading globals (see src/helpers/platform.ts),
// so there is nothing for jsdom to provide.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.spec.ts'],
  },
})

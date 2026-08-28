/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

// Injected by Vite `define` (see vite.config.ts).
declare const __APP_VERSION__: string
// Distinct from __APP_VERSION__: this one changes per build. See vite.config.ts.
declare const __BUILD_ID__: string

// No VITE_* app config here on purpose: runtime configuration comes from the BFF
// via GET /api/config (see src/config/runtime.ts), so one built image can be
// deployed with different values. Anything added to ImportMetaEnv is frozen into
// the bundle at build time.
interface ImportMetaEnv {
  readonly MODE: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

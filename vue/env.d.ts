/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

// Injected by Vite `define` (see vite.config.ts).
declare const __APP_VERSION__: string

interface ImportMetaEnv {
  /**
   * Dataforsyningen API token for the WMS base layers (PRD 002). A public quota
   * key rather than a credential, but still not committed: set it in
   * docker-compose.override.yml for dev and in the deploy env for prod.
   */
  readonly VITE_DATAFORSYNINGEN_TOKEN?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, unknown>, Record<string, unknown>, unknown>
  export default component
}

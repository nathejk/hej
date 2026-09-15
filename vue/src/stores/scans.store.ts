import { defineStore } from 'pinia'
import { fetchWrapper } from '@/helpers'

export type ScanKind = 'checkpoint' | 'bandit'

export interface Scan {
  id: string
  kind: ScanKind
  label: string
  /**
   * The post this scan happened at, or '' when the personnel rota could not place it.
   *
   * Empty is a normal outcome, not an error: a scan carries no checkpoint of its own and is attributed by
   * asking which post its scanner was on shift at. The map uses this to show a post as visited, so an
   * unattributed scan leaves that post looking unvisited — an under-statement rather than a wrong claim.
   */
  checkpointId: string
  /** null when the registration carries no position — listed, but not plotted. */
  lat: number | null
  lng: number | null
  scannedAt: Date
}

interface ScanResponse {
  id: string
  kind: ScanKind
  label: string
  checkpoint_id?: string
  lat: number | null
  lng: number | null
  scanned_at: string
}

// scans.store holds the signed-in user's patrol registrations: checkpoint scans
// and bandit catches. The BFF returns them newest-first and returns an empty list
// (not a 404) for users without a patrol — personnel roles — so an empty list is a
// normal state that simply hides the UI, not an error.
export const useScansStore = defineStore('scans', {
  state: () => ({
    scans: [] as Scan[],
    loading: false,
    loaded: false,
    error: '',
  }),
  getters: {
    /** Only the registrations we can put on the map. */
    positioned: (state) => state.scans.filter((s) => s.lat !== null && s.lng !== null),
    hasAny: (state) => state.scans.length > 0,
  },
  actions: {
    // fetch loads the patrol's registrations. Never throws: the map must stay
    // usable when this fails.
    async fetch() {
      this.loading = true
      try {
        const data = await fetchWrapper.get<{ scans: ScanResponse[] | null }>('/api/patrol/scans')
        this.scans = (data.scans ?? []).map((s) => ({
          id: s.id,
          kind: s.kind,
          label: s.label,
          // Optional in the response type so a client running against an older BFF keeps working.
          checkpointId: s.checkpoint_id ?? '',
          lat: s.lat,
          lng: s.lng,
          scannedAt: new Date(s.scanned_at),
        }))
        this.error = ''
        this.loaded = true
      } catch {
        this.error = 'Kunne ikke hente registreringer.'
      } finally {
        this.loading = false
      }
    },
  },
})

// The bottom overview handle on the map (PRD 016 §11.10, task 268).
//
// Extracted as a pure function because the rule it encodes is exactly the one the first cut got wrong: the
// handle used to appear only when there were registrations (`scans.hasAny`), so a patrol handed a map sheet
// but not yet scanned anything had no way to open the drawer and read their sticker numbers — the original
// problem this feature exists to solve. Pulling it out of the template lets a node test pin "visible when
// either list has something", without a DOM.

export interface DrawerHandle {
  /** Whether to show the handle at all. Hidden when there is nothing to open (personnel, or pre-race). */
  visible: boolean
  /** The label, covering both kinds. Empty when not visible. */
  label: string
}

/**
 * Decide the handle's visibility and label from the two counts.
 *
 * Each part appears only when it has something to count, joined with a middle dot. "kort" is invariant in
 * Danish; "registrering" takes an -er in the plural. Both zero means no handle, because opening an empty
 * drawer would be a dead end.
 */
export function drawerHandle(scanCount: number, handoutCount: number): DrawerHandle {
  const parts: string[] = []
  if (scanCount > 0) {
    parts.push(`${scanCount} ${scanCount === 1 ? 'registrering' : 'registreringer'}`)
  }
  if (handoutCount > 0) {
    parts.push(`${handoutCount} kort`)
  }
  return { visible: parts.length > 0, label: parts.join(' · ') }
}

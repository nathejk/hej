import type { Handout } from '@/stores/handouts.store'

// How a handout row reads in the drawer (PRD 016 §11.10, task 268).
//
// Small, but pure and tested for one reason above the others: the "no longer held" state must say
// "afleveret" and **name nobody**. HQ's organizer view shows "Flyttet til {team}"; a participant may not
// see which team a sheet moved to. Encoding the label here — a constant with no interpolation — makes that
// guarantee something a test can pin, rather than a property of markup that a later edit could quietly undo.

/**
 * The status line for a handout, or null when it is still held.
 *
 * Deliberately a bare constant: there is no team name, no id, nothing about where the sheet went — only
 * that the patrol no longer has it. A sheet still held needs no status line at all.
 */
export function handoutStatus(h: Pick<Handout, 'stillHeld'>): string | null {
  return h.stillHeld ? null : 'afleveret'
}

/**
 * The printed sticker number to show, or null when there is none.
 *
 * A synthesised handout (a skitse) has no code, so its row must not leave an empty gap where the number
 * would go — the caller renders the sticker line only when this is non-null.
 */
export function handoutSticker(h: Pick<Handout, 'qrId'>): string | null {
  return h.qrId === '' ? null : h.qrId
}

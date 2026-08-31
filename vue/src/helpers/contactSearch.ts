import type { ContactEntry } from '@/stores/contacts.store'

// Contacts search (PRD 007 §6, task 165).
//
// A pure function rather than logic inside the view, for the same reason `config/nudge.ts` is
// separate from the component that shows the nudge: the matching rules are worth testing, and
// `vitest.config.ts` runs in `node` with no jsdom, so anything worth testing has to be reachable
// without mounting a component.
//
// # It runs locally, always
//
// A search that needs the network is not a search, and this pane's premise is working at 03:00
// with no signal. At event size — a few hundred rows for the largest role — a linear scan is
// imperceptible, so there is no index to build and keep in sync.
//
// # Spejdere are not searchable, and that is structural
//
// This searches the synced directory, which by construction contains no spejder: the BFF never
// lists one. A patrol is found through its own explicit action (task 168), never by typing a
// number here — merging the two would make patrol numbers browsable by accident, which is the
// thing the whole design avoids.

/**
 * Folds a string for comparison: lower-cased, diacritics stripped, Danish letters normalised.
 *
 * So "soren" finds "Søren", "aerlige" finds "Ærlige", and "AA" finds "Å". Danish names are the
 * norm in this data, so accent-sensitive matching would fail on the common case rather than an
 * edge one.
 *
 * NFD splits a letter from its combining accent so the accent can be dropped, which handles é
 * and ü. ø and å do not decompose that way, and æ is a ligature rather than an accented letter,
 * so those three are mapped explicitly. Mapping them to `o`/`a`/`ae` matches how people
 * actually type when a keyboard makes the real letter awkward.
 */
export function foldForSearch(value: string): string {
  return value
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/ø/g, 'o')
    .replace(/æ/g, 'ae')
    .replace(/å/g, 'a')
}

/**
 * The text a person can be found by: name, crew function, and group labels.
 *
 * The phone number is deliberately **not** here — see `digitsOf`. Folding a number into the text
 * haystack means a two-character query like "30" matches every Danish mobile in the directory,
 * so typing the first two digits of a number shows the whole list. Keeping them apart lets a
 * name match on one character while a number needs enough digits to be a real question.
 */
function textOf(entry: ContactEntry): string {
  const parts = [entry.name, entry.crewFunction ?? '', entry.groups.map((g) => g.label).join(' ')]
  return foldForSearch(parts.join(' ')).replace(/\s+/g, ' ')
}

/** Just the digits of the stored number, so spacing in either the data or the query is moot. */
function digitsOf(entry: ContactEntry): string {
  return (entry.phone ?? '').replace(/\D/g, '')
}

/** Enough digits to be asking about a number rather than typing a name. */
const MIN_NUMBER_DIGITS = 3

/**
 * Returns the entries matching `query`, favourites first and then by name.
 *
 * An empty or whitespace-only query returns nothing rather than everything: the caller shows its
 * grouped sections in that case, and returning the whole directory would make the two states
 * indistinguishable to the view.
 *
 * Names, groups and crew functions match on any substring. Phone numbers match only once the
 * query carries at least three digits — "who was that missed call from?" is a real question
 * during an event, but it is not one anybody asks two digits at a time.
 */
export function searchContacts(
  entries: ContactEntry[],
  query: string,
  isFavourite: (id: string) => boolean = () => false,
): ContactEntry[] {
  const trimmed = query.trim()
  const needle = foldForSearch(trimmed).replace(/\s+/g, ' ')
  if (!needle) return []

  const queryDigits = trimmed.replace(/\D/g, '')
  const numberSearch = queryDigits.length >= MIN_NUMBER_DIGITS

  const matches = entries.filter((entry) => {
    if (textOf(entry).includes(needle)) return true
    if (numberSearch && digitsOf(entry).includes(queryDigits)) return true
    return false
  })

  return [...matches].sort((a, b) => {
    const fa = isFavourite(a.id)
    const fb = isFavourite(b.id)
    if (fa !== fb) return fa ? -1 : 1
    return a.name.localeCompare(b.name, 'da')
  })
}

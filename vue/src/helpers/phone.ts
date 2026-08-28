// Display formatting for phone numbers (PRD 003 §6).
//
// The *value* stays exactly as the BFF sent it — normalized E.164, which is what
// belongs in a `tel:` href. Only the label is prettified. Keeping those two apart
// matters: a "nicely formatted" href is what makes a dialer occasionally refuse a
// number.
//
// Danish numbers are eight digits, conventionally grouped in pairs: 12 34 56 78.
// Anything that is not a Danish number (a foreign guardian, a number we could not
// parse) is returned unchanged rather than forced into that shape — a wrong-looking
// number the user recognises is more useful than a plausible-looking wrong one.
export function formatPhone(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''

  const danish = trimmed.startsWith('+45')
    ? trimmed.slice(3)
    : /^\d{8}$/.test(trimmed)
      ? trimmed
      : ''

  const digits = danish.replace(/\s/g, '')
  if (!/^\d{8}$/.test(digits)) return trimmed

  return digits.replace(/(\d{2})(?=\d)/g, '$1 ')
}

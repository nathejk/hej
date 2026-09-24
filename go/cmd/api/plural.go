package main

import "strconv"

// Danish counts, in one place (task 387).
//
// # The bug this closes, and why one line was not the fix
//
// The frontpage's album card rendered `{{.Count}} billeder`, so an album holding one photograph read **"1
// billeder"**. Reported from the dev frontpage while verifying task 386.
//
// Small, and worth more than a shrug for two reasons. It is on the **public** page, the one surface read by
// families rather than by organizers; and a curator meets it on the first album they build, because an album
// starts with one photograph in it. It reads as carelessness about the whole page.
//
// # Why a helper, and not an `{{if}}` at the call site
//
// Task 387 said a helper only if there are genuinely several sites. There were fourteen. Six in Go, each with its
// own copy of
//
//	billeder := "billeder"
//	if n == 1 {
//		billeder = "billede"
//	}
//
// and three in templates, plus eight in JavaScript. Fourteen copies of one grammatical fact is how the fifteenth
// gets it wrong — which is exactly what happened on the frontpage, the only one of them ever written without the
// `if`.
//
// So the fix is structural: one definition, called from Go and registered as a template function, and the
// frontpage stops being able to get this wrong rather than merely being correct today.
// `TestNothingWritesTheDanishPluralByHand` is what keeps it that way.
//
// # Why these are not a general pluralise()
//
// Danish inflection does not reduce to a suffix rule, and a generic helper would invite exactly that. `album` and
// `glimt` are **invariant** — "1 album", "12 album" — so a rule that appended anything would break two of the
// three nouns this page counts. Naming each noun keeps the irregularity where it belongs: in the language, not in
// a caller's argument.

// albumCount renders a count of albums: "1 album", "12 album".
//
// **`album` is invariant in Danish**, so this exists to say so once rather than to inflect anything. Without it,
// the next person to count albums has to know that — and the safe assumption, by analogy with `billede`, is
// wrong. A function that looks pointless is cheaper than a page that reads "12 albummer".
func albumCount(n int) string {
	return strconv.Itoa(n) + " album"
}

// photoCount renders a count of photographs: "1 billede", "12 billeder".
//
// The number is included rather than returned beside the word, because that is the unit a sentence needs and
// splitting them is how a caller ends up re-deriving the singular to decide the article.
func photoCount(n int) string {
	if n == 1 {
		return "1 billede"
	}
	return strconv.Itoa(n) + " billeder"
}

// photoPronoun renders the pronoun a sentence about n photographs continues with: "det" or "de".
//
// **The count is only half of agreement, and the half that is easy to forget.** "Position sat på 1 billede. *De*
// vises på kortet." was still wrong after `photoCount` had fixed the first clause — found by reading the live
// message for a one-photograph selection rather than by looking for it. Worth remembering: a plural is not
// finished when the noun agrees.
//
// Lowercase, because that is the form inside a sentence. A caller starting one wraps it in `upperFirst`.
func photoPronoun(n int) string {
	if n == 1 {
		return "det"
	}
	return "de"
}

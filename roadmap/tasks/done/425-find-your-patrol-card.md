# 425 — "Find din patrulje" is a card, with a brand-yellow button

**Status:** done
**Priority:** low
**Created:** 2026-09-25
**Picked up by:** agent session (Zed)
**Started:** 2026-09-25
**Completed:** 2026-09-25

## Description

> *"The 'Find din patrulje' section on the year-view should be embedded in a card — slight grey background, a thin
> border and rounded corners. Instead of the blue button, the button should be dark yellow — yellow from logo, but
> darker."*

The section is now a panel: `#fafafa`, a hairline `#e4e4e7` border, `.5rem` corners. It opts **out** of the
section divider rather than sitting under one — a card below a rule reads as two boundaries for one edge.

It is also the right section to single out: the others are things to look at, this one has a field in it, and a
panel is how a page says "do something here" without a heading having to.

### Two decisions inside a styling change

**Both colours are set on the card, not just the background.** The page declares a light-dark colour scheme, so a
browser in dark mode supplies light text — and light text on a light card is an invisible form. The takedown
form's thanks panel is the other light island here and does the same thing for the same reason.

**The button's label is near-black, not white.** The logo's `#E6EA08` is unusable behind text at full brightness;
this is it at about 70%, which is still recognisably the same yellow. Against that, white lands around **2.7:1** —
below the 4.5:1 that a form's only button has to clear — while near-black is about **7.7:1**. Going dark enough
for white text would take it to olive, which is no longer the logo's yellow. So the colour is the brand's and the
contrast is the label's job.

## Acceptance Criteria

- [x] The section reads as a card: light grey, thin border, rounded
- [x] No section divider above it
- [x] The button is the logo's yellow, darkened, and its label clears 4.5:1 against it
- [x] The card is legible whatever colour scheme the browser supplies

## Progress Log

- 2026-09-25 — Done as described. The input gains explicit colours too, for the same dark-mode reason as the card.
- 2026-09-25 — No test. A colour is a judgement and a guard would only restate the value; what would be worth
  asserting is the contrast ratio, and that is not something this suite can compute from a stylesheet. The
  reasoning is in the CSS instead, next to the number it justifies.
- 2026-09-25 — **Noticed and deliberately not changed:** `.report button` in the footer is still `#1d4ed8`, which
  is a dark blue on the footer's dark grey — poor contrast, and it moved there in task 423 rather than being
  designed for it. Left alone because it is not what was asked for and a footer button is a different decision
  from a form's primary action; worth its own look.
- 2026-09-25 — ✅ All criteria met. `gofmt`, `go vet`, full `go test ./cmd/api/` clean.

# Mobile-only checklist

What the dev simulation layer (PRD 014) **cannot** cover, and therefore has to be checked
on real hardware before a release that touches any of it.

This list is the counterweight to that layer. A simulated iPhone is not an iPhone, and the
failure mode of a *good* simulation is false confidence: something that works on a laptop
and not in a forest at midnight. Everything below is either a capability a desktop browser
does not have, or a question about how the app *feels* in a hand.

## How this differs from task 139's device matrix

They overlap and are not the same list, so pick the right one:

- **Task 139's matrix** is about **classification** — does this device read as mobile, as
  standalone, as a webview? Its failures look like "an iPad was sent to the desktop
  placeholder". Three of its rows are now automated by
  `vue/scripts/check-install-gate.sh`.
- **This list** is about **capability and feel** — does the thing actually work, and is it
  usable? Its failures look like "notifications never arrive" or "the button is under my
  thumb".

If a failure is "the app decided I was the wrong kind of device", it belongs to 139. If it
is "the app knew what I was and the feature still did not work", it belongs here.

## The checklist

| Area | What to establish | Device |
|---|---|---|
| **Add to Home Screen** | The Share → Add to Home Screen flow completes, and the installed launcher icon is the maskable one, not the favicon. | iOS Safari |
| **Standalone chrome** | The launch splash uses the brand background (no white flash), the status bar is legible over the app bar, and nothing important sits under the notch or the home indicator. | iOS, installed |
| **iOS Web Push** | Permission can be granted **from the installed app**, a push arrives with the app backgrounded, and tapping it deep-links to the right route. This is the single most important row: iOS grants Web Push only to home-screen web apps on 16.4+, and it cannot be approximated on a laptop at all. | iOS 16.4+, installed |
| **Android install prompt** | `beforeinstallprompt` is captured, the richer install dialog shows name/description/screenshot, and `appinstalled` is observed. | Android Chrome |
| **Android push** | As iOS, plus that notifications survive the app being swiped away. | Android, installed |
| **Real GPS** | Accuracy varies plausibly, indoor loss is handled, the "stuck" timeout fires when no strategy answers, and the accuracy circle matches reality rather than the fake provider's tidy 12 m. | both |
| **Backgrounding** | Sampling stops when hidden and resumes on return; a track survives iOS **killing** a backgrounded standalone app, which it does aggressively (see the note in `location.store`). | iOS, installed |
| **Camera** | The portrait capture frames a face sensibly on a front camera, in poor light, and the result is legible at directory thumbnail size. | both |
| **Offline for real** | Airplane mode mid-event: tiles serve from cache, the directory renders, the offline notice is honest, and nothing hangs waiting on a request. Not the same as the panel's force-offline, which fails requests instantly rather than slowly. | both |
| **Patchy coverage** | One bar, high latency: the app must degrade rather than appear broken. This is the state the whole offline design exists for and the hardest to simulate — a train or a rural drive is the usual way to get it. | both |
| **Ergonomics** | One-handed reach for the bottom nav and the SOS route; tap targets under a gloved or cold thumb. | both |
| **Sunlight** | Legibility outdoors at midday and at night with dark adaptation. The map's base layers are the risk. | both |

## When to run it

Not every release. The rows are cheap to skip and expensive to skip *silently*, so the rule
is by area: touch push, run the push rows; touch the map or `track.store`, run the GPS and
backgrounding rows; touch the shell, layout or safe areas, run the standalone-chrome row.

Record outcomes in the relevant task's progress log — including passes, because "verified
on an iPhone 16, 2026-09" is the information a later reader actually needs.

#!/bin/sh
# Checks the install gate (PRD 005) in a real browser, for the cases a real browser can decide.
#
# Task 139's matrix is mostly device-only, and this does not replace it — see "NOT COVERED" below.
# What it does replace is the part of it that was being re-checked by hand on a laptop every time
# the gate changed, which is the part that broke twice in one week (tasks 141 and 142) and would
# have been caught here in seconds.
#
# Requires the dev stack up (docker compose up -d) and reachable on https://hej.local.nathejk.dk.
# --ignore-certificate-errors is for the local self-signed Traefik cert.
#
# WHY USER-AGENT SWITCHING IS ENOUGH FOR THESE CASES, and where it stops being enough:
#
#   `isMobileDevice()` checks the Apple UA patterns before it looks at touch capability, so an
#   iPhone/iPad UA classifies as mobile in a plain headless Chrome. `installPlatform()` is
#   UA-driven throughout. Default headless Chrome is a genuine mouse-only desktop, so the desktop
#   case is real rather than emulated.
#
#   It stops at anything the platform has to *tell* us: display-mode, navigator.standalone,
#   beforeinstallprompt, maxTouchPoints. Those need the real thing.
#
#   ANDROID CANNOT BE REPRESENTED HERE, and finding that out was worth the attempt. An Android UA
#   plus headless Chrome is still a mouse-only, touch-less desktop, and `isMobileDevice()` only
#   treats the *Apple* UA patterns as decisive — for everything else it requires touch or pointer
#   evidence, because `navigator.userAgentData.mobile` is false on an Android *tablet* and a UA
#   string is not evidence of hardware. So an Android UA classifies as desktop here and lands on
#   the website, which is the code behaving correctly on the inputs it was given.
#
#   That asymmetry is deliberate (see the comments in helpers/platform.ts) and it means **Android
#   must be tested on Android**. Not asserted below: pinning "Android UA → desktop" as expected
#   behaviour would enshrine something we do not actually want, and would fail the day someone
#   sensibly makes the UA decisive for Android too.
#
#   iPadOS is unrepresentable for the same reason, and it is the highest-risk row: its whole
#   difficulty is that it sends a *desktop* UA and is only detectable by touch points.
#
# NOT COVERED, and still on task 139 for a human with hardware:
#   * iOS Safari: add-to-home-screen, navigator.standalone, Web Push on 16.4+
#   * Android Chrome and Android Firefox: everything — see above, the UA alone is not enough
#   * iPadOS: reports as macOS Safari, must still classify as mobile
#   * in-app webview: that "reopen in Safari/Chrome" is followable
#   * camera, notch/safe-area, keyboards, light/dark rendering
set -eu

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
BASE="${BASE:-https://hej.local.nathejk.dk}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

[ -x "$CHROME" ] || { echo "Google Chrome not found at $CHROME" >&2; exit 1; }

UA_IPHONE="Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
UA_FACEBOOK="Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/440.0.0.32.108]"

failures=0
dom=""

# dump <name> <url> [user-agent]
#
# Chrome writes the DOM and then hangs on shutdown rather than exiting, so it is backgrounded,
# given time to settle, and killed — the same dance as capture-screenshots.sh. Each run gets a
# throwaway profile, which also guarantees an empty origin: no leftover session, no service
# worker, and no localStorage. That matters more here than anywhere else in this repo, because
# the gate reads persisted state and a stale profile would quietly invalidate every assertion.
dump() {
  name="$1"; url="$2"; ua="${3:-}"
  dom="$TMP/$name.html"
  if [ -n "$ua" ]; then
    "$CHROME" --headless --disable-gpu --no-first-run --ignore-certificate-errors \
      --user-data-dir="$TMP/profile-$name" --user-agent="$ua" \
      --virtual-time-budget=6000 --dump-dom "$url" > "$dom" 2>/dev/null &
  else
    "$CHROME" --headless --disable-gpu --no-first-run --ignore-certificate-errors \
      --user-data-dir="$TMP/profile-$name" \
      --virtual-time-budget=6000 --dump-dom "$url" > "$dom" 2>/dev/null &
  fi
  pid=$!
  sleep 9
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  [ -s "$dom" ] || { echo "  FAIL: no DOM captured for $name"; failures=$((failures + 1)); }
}

want() {
  if grep -q "$1" "$dom"; then
    echo "  ok      contains: $2"
  else
    echo "  FAIL    missing:  $2"
    failures=$((failures + 1))
  fi
}

reject() {
  if grep -q "$1" "$dom"; then
    echo "  FAIL    present but must not be: $2"
    failures=$((failures + 1))
  else
    echo "  ok      absent:   $2"
  fi
}

echo "== desktop: leaves the app for the anonymous website =="
dump desktop "$BASE/"
want "more to come" "the website placeholder"
# The banner is display:none in CSS and only revealed by an inline style, so an inline
# "display: block" here would mean a mouse-only desktop was invited to install a phone app.
reject 'id="install-banner" style="display: block' "the install CTA (mobile only)"
reject "Log ind" "any login form"

echo "== desktop: /welcome and /install are unreachable =="
dump desktop-welcome "$BASE/welcome"
want "more to come" "the website placeholder"
dump desktop-install "$BASE/install"
want "more to come" "the website placeholder"

echo "== iPhone browser tab: the install wall, iOS instructions =="
dump iphone "$BASE/" "$UA_IPHONE"
want "Tilf" "the add-to-home-screen instructions"
want "hjemmesk" "the home-screen wording"
want "hjemmesiden" "the link out to the website"
# No beforeinstallprompt on iOS, ever — the one-tap button must not be offered.
reject "Install.r app" "the one-tap install button"
# PRD 005 §11 (task 143): there is no login outside the installed app.
reject "Log ind" "any login form"
reject "Telefonnummer" "any phone-number field"

echo "== iPhone browser tab: /welcome is unreachable =="
dump iphone-welcome "$BASE/welcome" "$UA_IPHONE"
want "hjemmesk" "the install wall"
reject "Log ind" "any login form"

echo "== Facebook in-app webview (iOS): told to leave the webview =="
# Representable only because Facebook's iOS webview sends an iPhone UA, so the Apple path
# classifies it as mobile. The equivalent Android webview is not — see the header.
dump webview "$BASE/" "$UA_FACEBOOK"
want "bn siden i din browser" "the reopen-in-a-real-browser heading"
# Add-to-home-screen does not exist in a webview, so those steps must not be shown: they would
# send the user hunting for a control that is not there.
reject "Tilf.j til hjemmesk" "add-to-home-screen steps"
reject "Log ind" "any login form"

echo
if [ "$failures" -eq 0 ]; then
  echo "All install-gate checks passed."
else
  echo "$failures check(s) failed." >&2
  exit 1
fi

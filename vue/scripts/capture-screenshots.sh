#!/bin/sh
# Captures the manifest `screenshots` images into vue/public/screenshots/.
#
# These drive the richer install dialog Chromium shows on Android and desktop
# (name + description + preview, instead of a bare favicon and URL). Chrome wants
# at least one `narrow` and one `wide` entry, and the declared `sizes` in the
# manifest must match the files exactly or the whole set is ignored.
#
# Requires the dev stack up (docker compose up -d) and reachable on
# https://hej.local.nathejk.dk. --ignore-certificate-errors is for the local
# self-signed Traefik cert.
#
# NOTE: only /login is publicly reachable — every other route is behind the auth
# guard, and capturing maps/rulebook/contacts needs a seeded person plus a live
# jetstream to project them. See the comment in the capture list below.
set -eu

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
BASE="https://hej.local.nathejk.dk"
OUT="$(cd "$(dirname "$0")/../public" && pwd)/screenshots"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

[ -x "$CHROME" ] || { echo "Google Chrome not found at $CHROME" >&2; exit 1; }
mkdir -p "$OUT"

# Chrome writes the screenshot then hangs on shutdown rather than exiting, so it
# is backgrounded, polled for a size-stable file, and killed. Each capture gets a
# throwaway profile — both to avoid the profile lock blocking the next Chrome and
# to guarantee an empty origin (no leftover session or cached SW).
#
# WIDTHS BELOW ~500 DO NOT WORK, and fail deceptively. Chrome 132+ dropped the old
# headless implementation, so headless now drives a real browser window and
# enforces the platform's minimum window size: asking for 412 lays the page out at
# roughly 474 CSS px and then crops the screenshot to 412, so the result is the
# right size but the wrong framing — the centred login card sits visibly
# off-centre with its right edge cut. --force-device-scale-factor=2 does not help;
# the clamp is applied to the CSS width, not the physical one. Hence the narrow
# capture is 540 wide rather than a true 412 phone viewport. Verify any new size
# by eye before trusting it.
capture() { # <path> <width> <height> <output.png>
  dest="$OUT/$4"
  rm -f "$dest"
  "$CHROME" --headless --disable-gpu --hide-scrollbars \
    --no-first-run --no-default-browser-check --disable-extensions \
    --disable-background-networking --disable-sync \
    --ignore-certificate-errors \
    --user-data-dir="$TMP/profile-$4" \
    --window-size="$2,$3" \
    --virtual-time-budget=6000 \
    --screenshot="$dest" \
    "$BASE$1" >/dev/null 2>&1 &
  pid=$!

  i=0
  while [ $i -lt 150 ]; do
    if [ -s "$dest" ]; then
      a=$(wc -c < "$dest")
      sleep 0.2
      b=$(wc -c < "$dest")
      if [ "$a" = "$b" ]; then break; fi
    fi
    sleep 0.2
    i=$((i + 1))
  done

  kill -9 "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true

  [ -s "$dest" ] || { echo "failed to capture $1" >&2; exit 1; }
  echo "  screenshots/$4 (${2}x${3})"
}

curl -sk -o /dev/null "$BASE/" || { echo "$BASE unreachable — is the dev stack up?" >&2; exit 1; }

echo "Capturing screenshots -> vue/public/screenshots/"

# Only the login view for now. The interesting screens (maps, rulebook,
# contacts) sit behind the router's auth guard, and reaching them needs a session
# for a seeded test person — which needs jetstream up so the person projection is
# populated. Add them here once that is available; the manifest entries are
# already shaped to take more.
capture /login  540 1080 login-narrow.png
capture /login 1280  800 login-wide.png

echo "Done."

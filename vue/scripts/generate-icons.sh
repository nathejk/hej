#!/bin/sh
# Regenerates the raster PWA icons in vue/public/ from the vector sources in
# vue/src/assets/brand/.
#
# Run this after editing any brand SVG (e.g. correcting the moon's yellow) and
# commit the resulting PNGs — they are build inputs, not build outputs, so they
# are checked in and vite-plugin-pwa copies them straight through.
#
# Rasterises with headless Chrome rather than ImageMagick/rsvg/Ghostscript
# because none of those are installed on the team's machines, whereas Chrome is,
# and it is the same renderer that ultimately consumes the icons.
set -eu

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
MASTER=1024 # see rasterise()
BRAND="$(cd "$(dirname "$0")/../src/assets/brand" && pwd)"
OUT="$(cd "$(dirname "$0")/../public" && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

[ -x "$CHROME" ] || { echo "Google Chrome not found at $CHROME" >&2; exit 1; }

# Renders one SVG to a single large master PNG.
#
# Always at MASTER resolution, never directly at a target size, because macOS
# Chrome silently clamps --window-size to a platform minimum: asking for 96x96
# lays the page out at roughly 500px and then crops the screenshot to 96x96, so
# the result is the top-left corner of an oversized render — a plain background
# square, with the artwork off-frame. It fails quietly, producing a correctly
# sized, entirely wrong file. Downscaling from one master also antialiases the
# curves better than laying out at small sizes.
#
# Chrome writes the screenshot and then hangs on shutdown instead of exiting, so
# this backgrounds it, polls for a size-stable file, and kills it. Each render
# gets a throwaway profile; reusing one makes the next Chrome block on the lock.
rasterise() { # <source.svg> <master.png>
  dest="$TMP/$2"
  "$CHROME" --headless --disable-gpu --hide-scrollbars \
    --no-first-run --no-default-browser-check --disable-extensions \
    --disable-background-networking --disable-sync \
    --default-background-color=00000000 \
    --user-data-dir="$TMP/profile-$2" \
    --window-size="$MASTER,$MASTER" \
    --screenshot="$dest" \
    "file://$BRAND/$1" >/dev/null 2>&1 &
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
  [ -s "$dest" ] || { echo "failed to rasterise $1" >&2; exit 1; }
}

emit() { # <master.png> <size> <output.png>
  cp "$TMP/$1" "$OUT/$3"
  sips --resampleHeightWidth "$2" "$2" "$OUT/$3" >/dev/null
  echo "  $3 (${2}x${2})"
}

echo "Rendering icons -> vue/public/"

rasterise icon.svg icon.png
emit icon.png 512 pwa-512.png
emit icon.png 192 pwa-192.png
# iOS ignores SVG for apple-touch-icon, so this PNG is not optional. 180 is the
# @3x size; iOS downscales it for every other slot.
emit icon.png 180 apple-touch-icon.png

rasterise icon-maskable.svg maskable.png
emit maskable.png 512 maskable-512.png

rasterise badge.svg badge.png
emit badge.png 96 badge-96.png

echo "Done."

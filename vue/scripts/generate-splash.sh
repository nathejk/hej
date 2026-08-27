#!/bin/sh
# Generates the iOS launch ("boot") images in vue/public/splash/, plus the
# <link rel="apple-touch-startup-image"> block that must go in index.html.
#
# WHY THIS EXISTS: iOS does not use the manifest's `background_color` for the
# launch screen. With no startup image it shows plain white — which is what task
# 091 shipped, having wrongly assumed manifest support on iOS 16.4+. These images
# are the only way to get a branded boot screen on iPhone/iPad.
#
# iOS picks the image whose media query matches the device *exactly* and shows
# white if none does, so this has to be an exhaustive set rather than one or two
# sizes. Hence ~20 device configurations x 2 orientations.
#
# Note `device-width`/`device-height` describe the screen and do NOT swap with
# orientation — only the `orientation` term and the image's pixel dimensions do.
#
# After running this, delete and re-add the home-screen app: iOS caches the
# manifest, icons and startup images at install time and will not pick up changes
# otherwise.
set -eu

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
BRAND="$(cd "$(dirname "$0")/../src/assets/brand" && pwd)"
OUT="$(cd "$(dirname "$0")/../public" && pwd)/splash"
# Deliberately NOT written into public/: it is a copy-paste aid for index.html,
# not an asset, and anything under public/ gets deployed verbatim.
LINKS="$BRAND/startup-links.html"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

[ -x "$CHROME" ] || { echo "Google Chrome not found at $CHROME" >&2; exit 1; }
mkdir -p "$OUT"

# Device configurations: <css-width> <css-height> <dpr> <comment>
#
# One entry per unique (width, height, dpr) triple — several models share a
# triple (e.g. iPhone 14 Pro / 15 / 15 Pro / 16 are all 393x852@3) and a
# duplicate media query would be dead weight.
#
# Physical image size is css * dpr. Known gap: models released after this list was
# written (iPhone 17 and later) will fall back to white until their dimensions are
# added here.
DEVICES="
375 667 2 iPhone-SE-8-7-6s
414 736 3 iPhone-8Plus-7Plus-6sPlus
375 812 3 iPhone-X-XS-11Pro-12mini-13mini
414 896 2 iPhone-XR-11
414 896 3 iPhone-XSMax-11ProMax
390 844 3 iPhone-12-12Pro-13-13Pro-14
428 926 3 iPhone-12ProMax-13ProMax-14Plus
393 852 3 iPhone-14Pro-15-15Pro-16-16e
430 932 3 iPhone-14ProMax-15Plus-15ProMax-16Plus
402 874 3 iPhone-16Pro
440 956 3 iPhone-16ProMax
744 1133 2 iPad-mini-6-7
768 1024 2 iPad-9.7-mini4-5-Air1-2
810 1080 2 iPad-10.2
820 1180 2 iPad-10th-Air4-5-Air11-M2
834 1112 2 iPad-Pro-10.5-Air3
834 1194 2 iPad-Pro-11
834 1210 2 iPad-Pro-11-M4
1024 1366 2 iPad-Pro-12.9
1032 1376 2 iPad-Pro-13-M4
"

# Renders splash.html at an exact pixel size.
#
# Deliberately uses --window-size in *physical* pixels with the default scale
# factor rather than CSS points with --force-device-scale-factor. Two reasons:
# the layout is sized entirely in vmin so it is resolution-independent and comes
# out identical either way; and headless Chrome clamps window widths below ~500,
# silently cropping instead of scaling, which every phone's CSS width would hit.
#
# Chrome also writes the screenshot and then hangs instead of exiting, so it is
# backgrounded, polled for a size-stable file, and killed.
render() { # <pixel-width> <pixel-height> <output.png>
  dest="$OUT/$3"
  rm -f "$dest"
  "$CHROME" --headless --disable-gpu --hide-scrollbars \
    --no-first-run --no-default-browser-check --disable-extensions \
    --disable-background-networking --disable-sync \
    --user-data-dir="$TMP/p-$3" \
    --window-size="$1,$2" \
    --screenshot="$dest" \
    "file://$BRAND/splash.html" >/dev/null 2>&1 &
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
  [ -s "$dest" ] || { echo "failed to render $3" >&2; exit 1; }
}

emit_link() { # <device-width> <device-height> <dpr> <orientation> <file>
  cat >> "$LINKS" <<EOF
    <link
      rel="apple-touch-startup-image"
      media="screen and (device-width: ${1}px) and (device-height: ${2}px) and (-webkit-device-pixel-ratio: ${3}) and (orientation: ${4})"
      href="/splash/${5}"
    />
EOF
}

echo "Generating iOS launch images -> vue/public/splash/"
: > "$LINKS"
count=0

echo "$DEVICES" | while read -r w h dpr label; do
  [ -n "${w:-}" ] || continue

  pw=$((w * dpr))
  ph=$((h * dpr))

  portrait="splash-${pw}x${ph}.png"
  landscape="splash-${ph}x${pw}.png"

  render "$pw" "$ph" "$portrait"
  emit_link "$w" "$h" "$dpr" portrait "$portrait"
  echo "  $portrait  ($label, portrait)"

  render "$ph" "$pw" "$landscape"
  emit_link "$w" "$h" "$dpr" landscape "$landscape"
  echo "  $landscape  ($label, landscape)"
done

echo
echo "Wrote link tags to src/assets/brand/startup-links.html — paste into"
echo "vue/index.html between the startup-image markers. Then delete and re-add"
echo "the home-screen app: iOS caches these at install time."

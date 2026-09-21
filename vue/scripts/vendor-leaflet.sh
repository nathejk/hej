#!/bin/sh
# Copies Leaflet's built assets into vue/public/vendor/ for the public map island
# (PRD 011 §8, task 342).
#
# # Why the assets are vendored rather than loaded from a CDN
#
# Three reasons, in order of weight:
#
#  1. **Reach.** The public pages exist so somebody on a poor connection or an
#     unusual network can see the event. A CDN is one more host that can be
#     blocked, slow, or unreachable — on a page whose entire virtue is not
#     depending on much.
#  2. **No third-party request from a page about children.** The public site
#     makes requests to our origin and to Dataforsyningen for map tiles, and
#     that is the whole list. Adding a CDN would put a third party in a position
#     to log who looked at which patrol's route.
#  3. **Integrity without ceremony.** Same-origin assets need no SRI hash to
#     maintain, and cannot change under us between a review and a deploy.
#
# # Why a script rather than a build step
#
# Same reasoning as generate-icons.sh: the output is a **build input**, checked
# in, so vite copies it straight through and a deploy needs no extra tooling.
# Run this after upgrading Leaflet and commit the result.
set -eu

HERE="$(cd "$(dirname "$0")" && pwd)"
SRC="$HERE/../node_modules/leaflet/dist"
OUT="$HERE/../public/vendor"

[ -d "$SRC" ] || { echo "leaflet not installed; run npm install first" >&2; exit 1; }

mkdir -p "$OUT"

# The minified build and its stylesheet. The unminified one and the source maps
# are deliberately not copied: this is a runtime asset, and a source map would
# roughly triple what a visitor on a phone downloads to see a map.
cp "$SRC/leaflet.js" "$OUT/leaflet.js"
cp "$SRC/leaflet.css" "$OUT/leaflet.css"

# Leaflet's CSS references these by relative path, so the layout under vendor/
# has to match what it expects.
mkdir -p "$OUT/images"
cp "$SRC/images/"*.png "$OUT/images/" 2>/dev/null || true

# The marker-cluster plugin: the one new dependency PRD 011 spends (task 342).
# Two stylesheets, both needed: MarkerCluster.css positions the cluster icons,
# MarkerCluster.Default.css is the shipped look for them. Without the second one
# a cluster renders as an unstyled number in the corner of a box.
CSRC="$HERE/../node_modules/leaflet.markercluster/dist"
[ -d "$CSRC" ] || { echo "leaflet.markercluster not installed; run npm install first" >&2; exit 1; }
cp "$CSRC/leaflet.markercluster.js" "$OUT/leaflet.markercluster.js"
cp "$CSRC/MarkerCluster.css" "$OUT/MarkerCluster.css"
cp "$CSRC/MarkerCluster.Default.css" "$OUT/MarkerCluster.Default.css"

node -e "process.stdout.write(require('$HERE/../node_modules/leaflet.markercluster/package.json').version)" \
	> "$OUT/leaflet.markercluster.version" 2>/dev/null || echo "unknown" > "$OUT/leaflet.markercluster.version"

# The version, recorded next to the assets so a reviewer can tell what is
# checked in without reading package.json — and so an upgrade that forgets to
# re-run this script is visible in a diff.
node -e "process.stdout.write(require('$HERE/../node_modules/leaflet/package.json').version)" \
	> "$OUT/leaflet.version" 2>/dev/null || echo "unknown" > "$OUT/leaflet.version"

echo "vendored leaflet $(cat "$OUT/leaflet.version") into public/vendor/"
ls -l "$OUT"

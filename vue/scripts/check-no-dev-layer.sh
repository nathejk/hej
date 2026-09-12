#!/bin/sh
# Asserts that the dev simulation layer (PRD 014) is absent from a production build.
#
# THIS IS NOT BELT-AND-BRACES. The regression it checks for has already happened once, during
# task 208: every use of the dev layer was correctly guarded with `import.meta.env`, and the
# production bundle still contained the simulation's fake user-agent strings and the panel's copy.
# `import.meta.env` guards make code *unreachable*; they do not remove code that something still
# statically imports. Nothing in the test suite noticed, and nothing could have — the guards were
# all present and correct.
#
# The fix was to move the layer into src/dev/ and reach it only through a dynamic import inside
# `if (import.meta.env.DEV)`, which Rollup folds away entirely. But that fix is a *convention* —
# "nothing in src/dev/ may be imported statically from product code" — and conventions decay
# silently. One careless `import { readDevDevice } from '@/dev/devDevice'` in a product module
# restores the leak with no test failure and no visible symptom.
#
# What is at stake: this layer can make the app believe it is a mobile device running installed,
# and can feed the map a fabricated position. In a safety app used by minors in a forest, shipping
# any of that is not a cosmetic bug.
#
# The Go side has the equivalent guarantee, asserted differently and in the right place for it:
# go/cmd/api/dev_test.go proves /api/dev/pin is not registered outside ENV=development.
#
# Usage: vue/scripts/check-no-dev-layer.sh [dist-dir]
#   With no argument it builds first. Pass a directory to scan an existing build.
set -eu

cd "$(dirname "$0")/.."

DIST="${1:-}"
if [ -z "$DIST" ]; then
  echo "==> building"
  npm run build >/dev/null
  DIST="dist"
fi

[ -d "$DIST" ] || { echo "no such directory: $DIST" >&2; exit 1; }

# One list, so that adding a dev feature without extending this check is awkward rather than easy.
# Each entry is a fixed string that exists ONLY in the dev layer. Chosen to be specific: a generic
# word like "dev" would match minified identifiers and produce false alarms, which is how a check
# like this gets disabled.
#
# Keep in sync when adding to src/dev/. If a new module has no distinctive string, that is itself
# worth noticing — it means the module is small enough to hide.
FINGERPRINTS="
hej.dev.
FBAN/FBIOS;FBAV
Android 14; Pixel 8
Mobile; rv:127.0
nokia3310
iphone-portrait
simulated — not this machine
/api/dev/pin
no pin issued yet
force offline
"

failures=0

# `while read` over a heredoc rather than `for` over the variable: the fingerprints contain spaces
# and word-splitting mangles them (the first attempt "passed" while checking nonsense fragments).
while IFS= read -r pattern; do
  [ -n "$pattern" ] || continue
  if grep -rqF -- "$pattern" "$DIST"; then
    echo "  FAIL  present in the bundle: $pattern"
    grep -rlF -- "$pattern" "$DIST" | sed 's/^/          /'
    failures=$((failures + 1))
  else
    echo "  ok    absent: $pattern"
  fi
done <<EOF
$FINGERPRINTS
EOF

# A chunk named after the dev tree would mean the dynamic import survived as a real (lazily
# loaded) module rather than being folded away. It would never be requested at runtime, but it
# would be *served* — and its contents are the whole point of this check.
if ls "$DIST"/assets 2>/dev/null | grep -qiE 'devpanel|devdevice|platformsim|devgeolocation|devplayback|bootstrap'; then
  echo "  FAIL  a chunk was emitted for src/dev/:"
  ls "$DIST"/assets | grep -iE 'devpanel|devdevice|platformsim|devgeolocation|devplayback|bootstrap' | sed 's/^/          /'
  failures=$((failures + 1))
else
  echo "  ok    no chunk emitted for src/dev/"
fi

echo
if [ "$failures" -eq 0 ]; then
  echo "Production bundle is free of the dev layer."
else
  echo "$failures dev-layer leak(s) found in $DIST." >&2
  echo "See PRD 014 §8: nothing in src/dev/ may be imported statically from product code." >&2
  exit 1
fi

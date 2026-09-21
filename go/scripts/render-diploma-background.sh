#!/bin/sh
# Rasterise the diploma artwork from its vector source (task 345).
#
# The artwork arrives as a print-ready A4 PDF from InDesign. The renderer needs a **raster** version for two
# reasons, and only one of them is about the PDF:
#
#   - `diploma.PDF` stamps the background as a full-page image. A vector overlay would be sharper, but it needs
#     `phpdave11/gofpdi` — a new module, and one with a history of trouble on the PDF 1.5 object streams this
#     file uses — where this needs nothing.
#   - `diploma.Thumbnail` decodes `Background()` as an image, because the patrol page shows the diploma as a
#     picture. That one cannot be served by a PDF at all.
#
# So the JPEG is the asset and the PDF is kept beside it as the source of truth. Re-run this when the artwork
# changes; it is deliberately **not** part of the build — a build step that needs a GUI framework to rasterise a
# once-a-year asset would be a bad trade.
#
# # Why qlmanage and sips
#
# They are macOS's own CoreGraphics renderer, already on every machine here, where Ghostscript, poppler and
# ImageMagick are on none of them (checked: host, api container, ui container). `qlmanage -t -s <px>` renders
# the page at exactly that pixel height, with no border and no drop shadow — verified by measuring the output:
# 2480x3508 for A4, which is 300 dpi to the pixel.
#
# `sips` then flattens the alpha channel and encodes JPEG. Quality 85: the artwork is a paper texture, i.e.
# noise, so it compresses poorly (1.4 MB) and hides artefacts well. A lower setting saves little — measured
# 1.1 MB at quality 70 — and this is a file people print.
set -eu

here=$(dirname "$0")
assets="$here/../internal/diploma/assets"
source_pdf="$assets/source/Diplom2026.pdf"
out="$assets/background-2026.jpg"

[ -f "$source_pdf" ] || { echo "no artwork at $source_pdf" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# 3508 px on the long edge = 297 mm at 300 dpi.
qlmanage -t -s 3508 -o "$work" "$source_pdf" >/dev/null 2>&1
rendered="$work/$(basename "$source_pdf").png"
[ -f "$rendered" ] || { echo "qlmanage rendered nothing" >&2; exit 1; }

sips -s format jpeg -s formatOptions 85 "$rendered" --out "$out" >/dev/null

# Fail loudly on a size that is not A4 at 300 dpi. fpdf stretches whatever it is given across the page, so a
# mis-sized background would be silently distorted and nobody would notice until a family printed one.
width=$(sips -g pixelWidth "$out" | awk '/pixelWidth/ {print $2}')
height=$(sips -g pixelHeight "$out" | awk '/pixelHeight/ {print $2}')
if [ "$width" != "2480" ] || [ "$height" != "3508" ]; then
	echo "expected 2480x3508 (A4 at 300 dpi), got ${width}x${height}" >&2
	exit 1
fi

echo "wrote $out (${width}x${height})"

#!/usr/bin/env bash
# Encodes the final export formats from recordings/raw-capture.webm (the
# single continuous Playwright recording produced by record-browser.mjs).
# All four formats are derived from that one source -- nothing here
# re-records or fabricates footage.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${script_dir}"

RAW="recordings/raw-capture.webm"
OUT="output"
mkdir -p "${OUT}"

[ -f "${RAW}" ] || { echo "FAIL: ${RAW} not found -- run record-browser.mjs first" >&2; exit 1; }

echo "== 1. Master MP4 (1920x1080, H.264, faststart) =="
ffmpeg -y -i "${RAW}" \
  -c:v libx264 -profile:v high -pix_fmt yuv420p -crf 18 -preset slow \
  -movflags +faststart \
  -an \
  "${OUT}/kubepreflight-v1-launch-16x9.mp4"

echo "== 2. Optimized GIF (readable highlight cut, 3.0s-16.0s: terminal -> findings -> report) =="
# 640x360 / 8fps / 128-color palette keeps this in the same size range as
# the other demo GIFs already committed under docs/assets/ (140-508 KB) --
# an earlier 960x540/12fps cut was 2.7 MB, well outside that precedent,
# for a duration this short that's resolution/fps to trim, not content.
ffmpeg -y -ss 3.0 -to 16.0 -i "${RAW}" \
  -vf "fps=8,scale=640:360:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=3" \
  -loop 0 \
  "${OUT}/kubepreflight-v1-launch-16x9.gif"

echo "== 3. Square 1080x1080 MP4 (center crop, not stretch) =="
ffmpeg -y -i "${RAW}" \
  -vf "crop=1080:1080:420:0" \
  -c:v libx264 -profile:v high -pix_fmt yuv420p -crf 18 -preset slow \
  -movflags +faststart \
  -an \
  "${OUT}/kubepreflight-v1-launch-1x1.mp4"

echo "== 4. Poster frame (t=15.0s -- BLOCKED / 75/100 / verified against real EKS visible) =="
ffmpeg -y -ss 15.0 -i "${RAW}" -frames:v 1 -update 1 \
  "${OUT}/kubepreflight-v1-launch-poster.png"

echo
echo "== Output sizes =="
ls -la "${OUT}"

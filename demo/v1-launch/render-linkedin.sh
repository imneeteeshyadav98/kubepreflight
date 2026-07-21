#!/usr/bin/env bash
# Encodes the standalone LinkedIn teaser exports from
# recordings/linkedin-raw-capture.webm, produced by:
#   VARIANT=linkedin BASE_URL=... OUT_DIR=./recordings node record-browser.mjs
# This is a separate 15.8s recording, not a re-cut of the 30s master --
# see render.sh for the master recording's exports (16:9/1:1/GIF/poster).
set -euo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd)"
cd "${script_dir}"

RAW="recordings/linkedin-raw-capture.webm"
OUT="output"
mkdir -p "${OUT}"

[ -f "${RAW}" ] || {
  echo "FAIL: ${RAW} not found -- run 'VARIANT=linkedin BASE_URL=... OUT_DIR=./recordings node record-browser.mjs' first" >&2
  exit 1
}

echo "== 1. LinkedIn teaser (1280x720, H.264, 24fps, faststart) =="
ffmpeg -y -i "${RAW}" \
  -vf "scale=1280:720:flags=lanczos,fps=24,format=yuv420p" \
  -c:v libx264 -profile:v high -pix_fmt yuv420p -crf 20 -preset slow \
  -movflags +faststart \
  -an \
  "${OUT}/kubepreflight-linkedin-launch-v2.mp4"

echo "== 2. Square 1080x1080 variant (scale-to-fit + letterbox, not a center crop) =="
# A 1080x1080 center crop of this 1920x1080 recording only shows the
# middle 1080px of a 1500px-wide terminal window / report layout -- tried
# it, and it cut off the CLUSTER field, the NO-GO badge, and the whole
# VERDICT column in the report scene. Scaling the full frame to fit within
# 1080 width and padding top/bottom (letterbox, not stretched) keeps every
# pixel of real content on screen, matching the ink background so the
# bars are invisible against the scene.
ffmpeg -y -i "${RAW}" \
  -vf "scale=1080:-2:force_original_aspect_ratio=decrease,pad=1080:1080:(ow-iw)/2:(oh-ih)/2:color=0x0a1211,fps=24" \
  -c:v libx264 -profile:v high -pix_fmt yuv420p -crf 20 -preset slow \
  -movflags +faststart \
  -an \
  "${OUT}/kubepreflight-linkedin-launch-v2-1x1.mp4"

echo "== 3. Poster frame (opening title card, t=0.8s -- stable, post fade-in, pre fade-out) =="
ffmpeg -y -ss 0.8 -i "${RAW}" -frames:v 1 -update 1 \
  "${OUT}/kubepreflight-linkedin-launch-v2-poster.png"

echo
echo "== Output sizes =="
ls -la "${OUT}"/kubepreflight-linkedin-launch-v2*

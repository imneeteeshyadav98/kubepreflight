#!/usr/bin/env bash
# Reproduces the quality/leak checks this demo's outputs were verified
# against before being accepted -- run after render.sh, before treating
# output/ as final.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${script_dir}"

need_cmd() { command -v "$1" >/dev/null 2>&1 || { echo "FAIL: required command not found: $1" >&2; exit 1; }; }
need_cmd ffprobe
need_cmd python3

fail=0
check() {
  local label="$1"
  local condition="$2"
  if [ "${condition}" = "0" ]; then
    echo "FAIL: ${label}"
    fail=1
  else
    echo "OK: ${label}"
  fi
}

echo "== Sensitive-identifier leak scan (source: evidence/, assets/, scripts) =="
if grep -rEqn 'arn:aws:|(^|[^0-9])[0-9]{12}([^0-9]|$)|ip-[0-9]+-[0-9]+-[0-9]+-[0-9]+\.(ec2|[a-z0-9.-]*compute)\.internal' \
  evidence assets record-browser.mjs render.sh 2>/dev/null | grep -v REDACTED; then
  echo "FAIL: unredacted AWS ARN/account-id/private-hostname pattern found in demo source"
  fail=1
else
  echo "OK: no unredacted ARN/account-id/private-hostname pattern in demo source"
fi
if grep -rlqE 'AKIA[0-9A-Z]{16}' . --include="*.mjs" --include="*.sh" --include="*.json" --include="*.html" 2>/dev/null; then
  echo "FAIL: AWS access key pattern found"
  fail=1
else
  echo "OK: no AWS access key pattern"
fi

echo
echo "== Output files present =="
for f in \
  output/kubepreflight-v1-launch-16x9.mp4 \
  output/kubepreflight-v1-launch-16x9.gif \
  output/kubepreflight-v1-launch-1x1.mp4 \
  output/kubepreflight-v1-launch-poster.png
do
  check "${f} exists" "$([ -s "${f}" ] && echo 1 || echo 0)"
done

echo
echo "== 16x9 MP4: format/duration/faststart =="
read -r codec w h pix < <(ffprobe -v error -select_streams v:0 -show_entries stream=width,height,codec_name,pix_fmt -of csv=p=0 output/kubepreflight-v1-launch-16x9.mp4 | tr ',' ' ')
dur=$(ffprobe -v error -show_entries format=duration -of csv=p=0 output/kubepreflight-v1-launch-16x9.mp4)
check "resolution is 1920x1080 (got ${w}x${h})" "$([ "${w}" = "1920" ] && [ "${h}" = "1080" ] && echo 1 || echo 0)"
check "codec is h264 (got ${codec})" "$([ "${codec}" = "h264" ] && echo 1 || echo 0)"
check "pixel format is yuv420p, web-compatible (got ${pix})" "$([ "${pix}" = "yuv420p" ] && echo 1 || echo 0)"
check "duration is 25-30s (got ${dur}s)" "$(python3 -c "print(1 if 25 <= float('${dur}') <= 31 else 0)")"
check "moov atom precedes mdat (faststart)" "$(python3 -c "
data = open('output/kubepreflight-v1-launch-16x9.mp4', 'rb').read(2_000_000)
moov, mdat = data.find(b'moov'), data.find(b'mdat')
print(1 if 0 < moov < mdat else 0)
")"

echo
echo "== 1x1 MP4: square resolution =="
read -r w h < <(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 output/kubepreflight-v1-launch-1x1.mp4 | tr ',' ' ')
check "resolution is 1080x1080 (got ${w}x${h})" "$([ "${w}" = "1080" ] && [ "${h}" = "1080" ] && echo 1 || echo 0)"

echo
echo "== GIF: readable size =="
gif_kb=$(( $(stat -c%s output/kubepreflight-v1-launch-16x9.gif) / 1024 ))
check "GIF under 1.5 MB (got ${gif_kb} KB)" "$([ "${gif_kb}" -lt 1536 ] && echo 1 || echo 0)"

echo
echo "== Poster: resolution =="
read -r w h < <(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 output/kubepreflight-v1-launch-poster.png | tr ',' ' ')
check "poster is 1920x1080 (got ${w}x${h})" "$([ "${w}" = "1920" ] && [ "${h}" = "1080" ] && echo 1 || echo 0)"

echo
if [ "${fail}" -ne 0 ]; then
  echo "VERIFY: FAILED -- see FAIL lines above"
  exit 1
fi
echo "VERIFY: all checks passed"

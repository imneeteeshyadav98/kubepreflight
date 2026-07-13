#!/usr/bin/env bash
set -euo pipefail

bench_count="${BENCH_COUNT:-3}"
bench_time="${BENCH_TIME:-1s}"
bench_package="${BENCH_PACKAGE:-./internal/integration}"
bench_regex="${BENCH_REGEX:-BenchmarkScale}"

cmd=(
  go test "${bench_package}"
  -run '^$'
  -bench "${bench_regex}"
  -benchmem
  -count "${bench_count}"
  -benchtime "${bench_time}"
)

echo "Running scale benchmarks:"
printf ' %q' "${cmd[@]}"
echo

if command -v /usr/bin/time >/dev/null 2>&1 && /usr/bin/time -v true >/dev/null 2>&1; then
  /usr/bin/time -v "${cmd[@]}"
else
  echo "GNU /usr/bin/time -v unavailable; peak RSS will not be reported." >&2
  "${cmd[@]}"
fi

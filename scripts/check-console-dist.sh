#!/usr/bin/env bash
# Release-hygiene guard, not a build step: rebuilds the Console and fails if
# the committed web/dist doesn't match what web/src currently produces. Go
# build the CLI, go:embed reads whatever is already on disk in web/dist —
# it has no way to notice that web/src changed since the last `npm run
# build`, so a stale dist silently ships an old Console with no error
# anywhere. Run this before committing any web/src change, and ideally wire
# it into CI once this project has one.
set -euo pipefail

cd "$(dirname "$0")/.."

npm --prefix web run build

if ! git diff --exit-code -- web/dist; then
  echo
  echo "web/dist is stale: 'npm run build' produced different output than what's committed." >&2
  echo "Review the diff above, then 'git add web/dist' and commit it alongside your web/src change." >&2
  exit 1
fi

echo "web/dist is up to date with web/src."

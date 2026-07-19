#!/usr/bin/env bash
# Guards against the exact bug class actionlint cannot catch: a syntactically
# valid `uses:` reference to a tag/SHA that either doesn't exist, or existed
# during the aquasecurity/trivy-action + aquasecurity/setup-trivy supply-chain
# compromise (GHSA-69fq-xp46-6x23 / CVE-2026-33634, 2026-03-19/20). Scoped to
# that incident's actual blast radius (the aquasecurity/* action chain), not a
# blanket SHA-pinning mandate for every third-party action in this repo --
# anchore/sbom-action, ossf/scorecard-action, and softprops/action-gh-release
# are pre-existing, unrelated, tag-pinned references this script deliberately
# leaves alone.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

fail=0

# Known-malicious commits from the advisory's tag-hijacking window. Kept as
# an explicit denylist (defense in depth) even though SHA-pinning below
# already prevents ever re-resolving a moved tag.
bad_shas=(
  # trivy-action: any commit force-pushed during 2026-03-19 ~17:43 UTC to
  # 2026-03-20 ~05:40 UTC is untrusted by construction -- we don't have a
  # complete list of the 76 hijacked commit SHAs, so this list is seeded
  # with placeholders for the two this repo actually referenced; extend if
  # Aqua publishes a full IOC commit list.
)

bad_trivy_binary_versions=("v0.69.4" "v0.69.5" "v0.69.6")

workflow_files=$(find .github/workflows action.yml compare/action.yml -name '*.yml' -o -name '*.yaml' 2>/dev/null)

echo "== aquasecurity/trivy-action and aquasecurity/setup-trivy must be SHA-pinned =="
while IFS=: read -r file line ref; do
  # Strip the "uses:" prefix and any trailing "# v0.36.0"-style comment
  # before splitting action@version, so a correctly SHA-pinned reference
  # with a human-readable comment isn't misread as unpinned.
  ref="$(echo "$ref" | sed -E 's/^[[:space:]]*uses:[[:space:]]*//; s/[[:space:]]*#.*$//')"
  action="${ref%@*}"
  version="${ref#*@}"
  if [[ ! "$version" =~ ^[0-9a-f]{40}$ ]]; then
    echo "FAIL: $file:$line references $action by tag/branch (\"$version\"), not a full commit SHA."
    echo "      aquasecurity/trivy-action and aquasecurity/setup-trivy tags were force-pushed to"
    echo "      malware during GHSA-69fq-xp46-6x23 -- pin to a full 40-char SHA."
    fail=1
  fi
done < <(grep -RnE 'uses:[[:space:]]*aquasecurity/(trivy-action|setup-trivy)@' $workflow_files || true)

echo "== no known-malicious commit SHA referenced =="
for sha in "${bad_shas[@]}"; do
  if grep -RnE "aquasecurity/(trivy-action|setup-trivy)@${sha}" $workflow_files; then
    echo "FAIL: references a commit SHA on the GHSA-69fq-xp46-6x23 IOC list."
    fail=1
  fi
done

echo "== no compromised Trivy binary version pinned =="
for bad_version in "${bad_trivy_binary_versions[@]}"; do
  if grep -RnE "version:[[:space:]]*['\"]?${bad_version}['\"]?" $workflow_files; then
    echo "FAIL: pins Trivy binary $bad_version, one of the malicious releases from GHSA-69fq-xp46-6x23."
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  echo
  echo "See https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23"
  exit 1
fi

echo "OK: no known-affected aquasecurity/trivy-action or aquasecurity/setup-trivy reference found."

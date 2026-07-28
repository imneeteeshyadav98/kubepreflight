#!/usr/bin/env bash
# Greps a directory of committed certification evidence for known-sensitive
# patterns and fails loudly if anything is found. Written for
# docs/certification/v1.3.0 (see that directory's environment.md,
# "Sanitization plan" section, for exactly why --redact-sensitive-identifiers
# alone is not trusted as sufficient before evidence is committed).
#
# This is a defense-in-depth check, independent of internal/redact's own
# regexes -- it deliberately does not shell into or import that Go package,
# so it still catches a regression even if redact.go itself stopped
# redacting something it used to. It also covers several identifier shapes
# internal/redact/redact.go does not cover at all: VPC/subnet/security-group
# IDs, private IPv4 addresses, the EKS cluster API endpoint hostname shape,
# AWS access key/session key IDs, secret access keys/session tokens (near a
# credential field name), bearer tokens, kubeconfig-embedded
# certificate/key data, and standalone STS assumed-role identifiers (not
# inside an ARN) -- see environment.md for the confirmed coverage gap this
# closes and Stage 2's independent review for why the credential/token/
# kubeconfig categories were added on top of Stage 1's original 8.
#
# Usage: check-evidence-sanitized.sh <directory> [<directory> ...]
#
# Exit 0: no matches found in any given directory (only .json/.md/.html/.txt
#         files are scanned -- binary files, if any, are skipped).
# Exit 1: at least one match found; every offending file:line is printed.
# Exit 2: usage error (no directory given, or a given path doesn't exist).
set -euo pipefail

if [[ $# -eq 0 ]]; then
  echo "usage: $(basename "$0") <directory> [<directory> ...]" >&2
  exit 2
fi

# Each entry is "label|extended-regex". Kept as parallel arrays (not an
# associative array) for portability with bash's default (non-4+) grep
# invocation style used below, and so match order in output is stable and
# predictable rather than hash-map-ordered.
#
# insensitive[i] (parallel array, "1" or "0") controls whether pattern i is
# matched case-insensitively. The original 8 patterns stay case-sensitive
# (unchanged behavior); patterns added afterward (Stage 2 independent review
# additions -- see approval-record.md/environment.md's sanitization section
# for why these were missing) opt into case-insensitive matching only where
# the real-world value legitimately varies in case (credential field names,
# "Bearer").
labels=(
  "AWS ARN"
  "12-digit AWS account ID"
  "VPC ID"
  "Subnet ID"
  "Security group ID"
  "EC2-style internal node hostname"
  "EKS cluster API endpoint hostname"
  "Private IPv4 address (RFC 1918)"
  "AWS access key ID / session key ID"
  "AWS secret access key or session token (near a credential field name)"
  "Bearer token"
  "Kubeconfig embedded certificate/key data"
  "STS assumed-role identifier (standalone, not inside an ARN)"
)
patterns=(
  'arn:aws:[a-zA-Z0-9][a-zA-Z0-9.-]*:[a-z0-9-]*:[0-9]{12}:[a-zA-Z0-9/:_.-]+'
  '\b[0-9]{12}\b'
  '\bvpc-[0-9a-f]{8,17}\b'
  '\bsubnet-[0-9a-f]{8,17}\b'
  '\bsg-[0-9a-f]{8,17}\b'
  '\bip-[0-9]{1,3}-[0-9]{1,3}-[0-9]{1,3}-[0-9]{1,3}\.(ec2\.internal|[a-zA-Z0-9-]+\.compute\.internal)\b'
  '[a-zA-Z0-9]+\.[a-zA-Z0-9]+\.[a-z0-9-]+\.eks\.amazonaws\.com'
  '\b(10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|172\.(1[6-9]|2[0-9]|3[0-1])\.[0-9]{1,3}\.[0-9]{1,3}|192\.168\.[0-9]{1,3}\.[0-9]{1,3})\b'
  '\b(AKIA|ASIA|AROA|AIDA|AGPA|ANPA|ANVA)[A-Z0-9]{16}\b'
  '(secret[_-]?access[_-]?key|session[_-]?token)["'"'"']?[[:space:]]*[:=][[:space:]]*["'"'"']?[A-Za-z0-9/+]{30,}'
  '[Bb]earer [A-Za-z0-9._-]{20,}'
  '(client-(certificate|key)-data:[[:space:]]*[A-Za-z0-9+/=]{20,}|-----BEGIN [A-Z ]*(PRIVATE KEY|CERTIFICATE)-----)'
  'assumed-role/[A-Za-z0-9+=,.@_-]+/[A-Za-z0-9+=,.@_-]+'
)
insensitive=(0 0 0 0 0 0 0 0 0 1 0 1 0)

found_any=0
scanned_files=0

for dir in "$@"; do
  if [[ ! -d "$dir" ]]; then
    echo "error: not a directory: $dir" >&2
    exit 2
  fi

  while IFS= read -r -d '' file; do
    scanned_files=$((scanned_files + 1))
    for i in "${!patterns[@]}"; do
      # grep -n prints matching lines with line numbers; -E extended regex;
      # -I skips binary files defensively even though the file list is
      # already extension-filtered; a non-zero exit from grep here just
      # means "no match in this file for this pattern" and must not abort
      # the loop under `set -e`, hence the explicit `|| true` + exit-code
      # check via a subshell-free `if`. -i is added only for patterns whose
      # insensitive[i] flag is "1" (see that array's own comment above).
      grep_flags="-nEI"
      if [[ "${insensitive[$i]:-0}" == "1" ]]; then
        grep_flags="-nEIi"
      fi
      if matches=$(grep $grep_flags "${patterns[$i]}" "$file" 2>/dev/null || true); [[ -n "$matches" ]]; then
        found_any=1
        echo "FOUND [${labels[$i]}] in $file:"
        echo "$matches" | sed 's/^/    /'
      fi
    done
  done < <(find "$dir" -type f \( -name '*.json' -o -name '*.md' -o -name '*.html' -o -name '*.txt' \) -print0)
done

echo ""
if [[ $found_any -eq 1 ]]; then
  echo "check-evidence-sanitized.sh: FAILED -- sensitive patterns found in ${scanned_files} scanned file(s) across: $*" >&2
  exit 1
fi

echo "check-evidence-sanitized.sh: OK -- ${scanned_files} file(s) scanned across: $*, no sensitive patterns found."
exit 0

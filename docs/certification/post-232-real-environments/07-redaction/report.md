# Redaction Certification

Date: 2026-07-29. All Lane 2 (real EKS) scans and Lane 3 rollback assessments were run with `--redact-sensitive-identifiers`.

## Source of truth

At the time of this certification, `internal/redact/redact.go` implemented exactly three patterns:

1. `arnPattern` — AWS ARNs (`arn:aws:...`)
2. `hostnamePattern` — EC2-style internal hostnames (`ip-A-B-C-D.ec2.internal` / `*.compute.internal`)
3. `accountIDPattern` — bare 12-digit AWS account IDs

## What was tested and the result

| Category | Tested against | Result |
|---|---|---|
| AWS account ID (12-digit) — file outputs | Real account ID appearing in `eksCluster.arn` and coverage error strings | ✅ Redacted in `findings.json`/`report.md`/`report.html` — 0 raw matches across all 6 IAM profiles |
| AWS ARNs — file outputs | Real cluster ARN, real IAM user/role ARNs in error messages | ✅ Redacted in `findings.json`/`report.md`/`report.html` — replaced with `[redacted-arn]` consistently |
| **AWS account ID / ARNs — terminal (`stdout`) output** | Same real identifiers, same run, `--terminal-output full` | ❌ **NOT redacted — confirmed real defect (3b below); 25 raw instances across 6 profiles' `stdout.txt`, despite `--redact-sensitive-identifiers` being passed and terminal being named explicitly in the flag's own `--help` text** |
| EC2-internal hostnames | Real node hostname (`ip-<private-ip>.ec2.internal`) | ✅ Redacted in file outputs — 0 raw matches |
| Private IP addresses (192.168.x.x, this cluster's VPC CIDR) | Real node private IP, appearing in `kubectl get nodes -o wide` and potentially in findings | ✅ 0 raw matches in any retained evidence file (not explicitly redacted by name — evidently never surfaced in scan output text in a form the sweep could find, or coincided with the hostname pattern's coverage) |
| Public IP addresses | Real node public IP | ✅ 0 raw matches in retained evidence |
| EKS API endpoint URLs | Real EKS API server endpoint | ✅ 0 raw matches |
| HTTP authorization token values / access keys / session tokens | N/A — kubeconfig auth token and IAM access keys never written into any findings/report output by design; independently confirmed absent | ✅ 0 raw matches |
| **VPC IDs (`vpc-...`)** | Real VPC ID appearing in `coverage.aws.errors[]` raw AWS SDK error strings | ❌ **NOT redacted — confirmed real defect, see below** |
| **Security Group IDs (`sg-...`)** | Real SG IDs (2 distinct) appearing in the same error strings | ❌ **NOT redacted — same defect** |
| Subnet IDs (`subnet-...`) | Not observed to appear in any output this session (this cluster's errors happened to reference VPC/SG, not subnets, in the specific denied calls exercised) | Not directly observed either way; same code-level gap applies by inspection (no subnet pattern exists in `redact.go`) |
| kubeconfig paths / local absolute paths | The certification's own working-directory paths, echoed by the CLI's "Reports written:" message | Not covered by product redaction (not a product concern — see below), manually sanitized in retained evidence for hygiene |

## Local fix status

Status after the coordinated defect-fix pass and follow-up real-EKS recertification: **fixed, real-EKS verified**.

The local fix expands the shared redaction policy beyond the original three patterns to include AWS infrastructure IDs, EKS API endpoint URLs, access-key/session-token/bearer-token values, IP addresses, bounded local paths, and rule-execution reasons. When `--redact-sensitive-identifiers` is enabled, terminal `stdout` renderings are covered for `scan`, `plan`, `compare`, `rollback plan`, and `rollback assess`; user-visible collector/configuration failures routed to `stderr` are also sanitized before printing. Scan and plan partial-collector terminal notices now pass cluster/AWS-derived error text through the same policy; rollback EKS collector failures are sanitized under the same flag.

Initially verified locally by unit tests and real-binary synthetic fixture checks, then re-certified against a fresh disposable EKS cluster after PR #235 merged. See `../09-real-eks-redaction-recertification/report.md`.

## Finding 3 (full detail) — two distinct, compounding gaps

### 3a. No redaction pattern exists for VPC/Security-Group/Subnet/Instance/Volume IDs (`RED-CLOUD-ID-002`, P2/Medium — see `../DEFECTS.md`)

Confirmed by direct code inspection: `internal/redact/redact.go` has exactly three patterns (ARN, EC2-hostname, 12-digit account ID) — nothing for `vpc-*`/`sg-*`/`subnet-*`/`i-*`/`vol-*`. Confirmed empirically: `grep -oE 'vpc-[0-9a-f]{6,}|sg-[0-9a-f]{6,}'` against `../02-reduced-iam-eks/profile-a/findings.json` and `profile-d/{findings.json,report.md,report.html}` returned three real, raw identifiers (one VPC ID, two Security Group IDs), embedded in the raw AWS SDK `403` error text inside `coverage.aws.errors[]`. This gap applies uniformly to every output format (JSON, Markdown, HTML, terminal) since there is simply no pattern to strip regardless of output path.

### 3b. Terminal (stdout) output is not redacted at all — even for patterns that DO exist (`RED-TERMINAL-001`, P1/High — the most important finding in this certification; see `../DEFECTS.md`)

This is the more serious half of the finding, discovered by re-checking every output format independently rather than trusting one file as representative. **`findings.json`, `report.md`, and `report.html` are all correctly redacted for ARNs and account IDs — 0 raw matches, confirmed across all 6 IAM profiles.** But the exact same run's raw terminal output (`stdout.txt`, captured with `--terminal-output full`) contains **unredacted, real ARNs and account IDs** — confirmed with a precise per-profile sweep:

| Profile | `findings.json` matches | `report.md` matches | `stdout.txt` matches |
|---|---:|---:|---:|
| A | 0 | 0 | 7 |
| B | 0 | 0 | 3 |
| C | 0 | 0 | 3 |
| D | 0 | 0 | 7 |
| E | 0 | 0 | 3 |
| F | 0 | 0 | 2 |

This directly contradicts the CLI's own documented promise: `scan --help`'s `--redact-sensitive-identifiers` description explicitly says *"in every output (findings.json, report.md/html, terminal output)"* — terminal is named explicitly and is the one format that does not actually get redacted. All 25 leaked instances across the 6 `stdout.txt` files were manually scrubbed from retained evidence before this report was finalized; the table above reflects the *pre-scrub* raw counts, captured before sanitization.

Severity: **Medium-High** (3b in particular — a documented, explicitly-named coverage promise that silently doesn't hold for one of the three named formats is a more concerning gap than an undocumented identifier category; an operator piping `--terminal-output full` into a CI log, exactly the documented use case, would leak real ARNs/account IDs believing the flag protected them). At certification time this was not fixed per certification scope; see `../DEFECTS.md` (`RED-TERMINAL-001`, `RED-CLOUD-ID-002`) for the current local-fix status and acceptance criteria.

## Redaction does not alter evaluation semantics

Confirmed by direct comparison of a redacted vs. equivalent non-redacted run's `findings[].fingerprint`, `severity`, `priority`, `upgradeReadiness.readinessScore`, `upgradeReadiness.verdict`, `ruleExecutions[].state`, and process exit code — all identical between the Lane 2 full-access baseline (non-redacted terminal preview during the run) and its redacted `findings.json` output. Per CLI documentation this is also an explicit, tested product guarantee (`--redact-sensitive-identifiers` help text: "does not change findings, scores, or exit codes"), consistent with what was observed.

# Windows equivalent of verify-published-binary.sh -- same checks, same
# order, so the Linux/macOS/Windows jobs in the published installation
# matrix (KP-V1-INSTALL-001) assert identical behavior across platforms
# instead of silently drifting apart. Run with `pwsh` (PowerShell Core),
# not Windows PowerShell 5.1.
#
# Usage: verify-published-binary.ps1 -BinPath <path> -Mode <light|deep> [-OutDir <dir>]
#
# Must be run from the repository root (needs testdata/manifest-repo/raw
# for the deep fixture scan) with $env:GITHUB_REF_NAME set.

param(
    [Parameter(Mandatory = $true)][string]$BinPath,
    [Parameter(Mandatory = $true)][ValidateSet('light', 'deep')][string]$Mode,
    [string]$OutDir = 'verify-scan-out'
)

$ErrorActionPreference = 'Stop'

if (-not $env:GITHUB_REF_NAME) {
    Write-Error 'GITHUB_REF_NAME is not set'
    exit 1
}

function Fail([string]$Message) {
    Write-Host $Message
    exit 1
}

Write-Host '== version banner matches this release =='
$expected = "KubePreflight $($env:GITHUB_REF_NAME.TrimStart('v'))"
$out = & $BinPath version
$outText = $out -join "`n"
Write-Host $outText

if ($out[0] -ne $expected) {
    Fail "want first line '$expected', got: $outText"
}
if ($outText -match '(?m)^commit: unknown$') {
    Fail "commit is 'unknown' -- ldflags did not reach this binary"
}
if ($outText -match '(?m)^built: unknown$') {
    Fail "built date is 'unknown' -- ldflags did not reach this binary"
}

$outVersionFlag = & $BinPath --version
$outVersionFlagText = $outVersionFlag -join "`n"
if ($outVersionFlagText -ne $outText) {
    Fail "kubepreflight version and --version disagree:`nversion:`n$outText`n--version:`n$outVersionFlagText"
}

Write-Host '== --help present on all five command surfaces =='
$surfaces = @('scan', 'plan', 'rollback plan', 'rollback assess', 'compare')
foreach ($surface in $surfaces) {
    $args = $surface.Split(' ') + '--help'
    & $BinPath @args | Out-Null
    if ($LASTEXITCODE -ne 0) {
        Fail "kubepreflight $surface --help failed"
    }
}

Write-Host '== --redact-sensitive-identifiers present on all five surfaces =='
foreach ($surface in $surfaces) {
    $args = $surface.Split(' ') + '--help'
    $helpText = (& $BinPath @args) -join "`n"
    if ($helpText -notmatch '--redact-sensitive-identifiers') {
        Fail "missing --redact-sensitive-identifiers on: kubepreflight $surface"
    }
}

if ($Mode -eq 'light') {
    Write-Host "OK (light): $BinPath"
    exit 0
}

Write-Host '== deep: deterministic fixture scan =='
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
& $BinPath scan `
    --target-version 1.34 `
    --manifests-only `
    --manifests testdata/manifest-repo/raw `
    --output all `
    --output-dir $OutDir `
    --redact-sensitive-identifiers `
    --serve-report never `
    --terminal-output silent
$code = $LASTEXITCODE
# Reset immediately after capturing it into $code. A nonzero $LASTEXITCODE
# left lingering from this scan (2, expected/intended) is otherwise still
# the last native-process exit code in scope when this script's own
# control flow later completes without an explicit `exit` -- pwsh's own
# step wrapper reads that stale value as the whole step's result, not
# this script's actual pass/fail logic. Confirmed as the real cause of a
# published-release run failing after printing "OK (deep)" as its last
# line: every check had genuinely passed, but the process exit code
# didn't reflect that.
$global:LASTEXITCODE = 0
# testdata/manifest-repo/raw/psp.yaml is a removed-API (policy/v1beta1
# PodSecurityPolicy) positive fixture -- same one scan_test.go locks a
# BLOCKED verdict to -- so this scan must always exit 2, deterministically.
if ($code -ne 2) {
    Fail "expected exit code 2 (BLOCKED) scanning the removed-API fixture, got $code"
}

Write-Host '== deep: artifacts exist, parse, and verdict is BLOCKED =='
foreach ($f in @('findings.json', 'report.md', 'report.html')) {
    $path = Join-Path $OutDir $f
    if (-not (Test-Path $path) -or (Get-Item $path).Length -eq 0) {
        Fail "missing $path"
    }
}
$findings = Get-Content (Join-Path $OutDir 'findings.json') -Raw | ConvertFrom-Json
if ($findings.upgradeReadiness.verdict -ne 'BLOCKED') {
    Fail "expected verdict BLOCKED, got $($findings.upgradeReadiness.verdict)"
}

Write-Host '== deep: redaction leak grep =='
$pattern = 'arn:aws:|(^|[^0-9])[0-9]{12}([^0-9]|$)|ip-[0-9]+-[0-9]+-[0-9]+-[0-9]+\.(ec2|[a-z0-9.-]*compute)\.internal'
$leaks = Get-ChildItem -Path $OutDir -Recurse -File | Select-String -Pattern $pattern
if ($leaks) {
    Write-Host ($leaks | Out-String)
    Fail 'sensitive identifier pattern found in redacted output'
}

Write-Host "OK (deep): $BinPath"
exit 0

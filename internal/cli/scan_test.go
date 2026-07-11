package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/findings"
)

func TestNormalizeNamespaceAllowlist(t *testing.T) {
	got, err := normalizeNamespaceAllowlist([]string{"payments", " platform ", "payments"})
	if err != nil {
		t.Fatalf("normalizeNamespaceAllowlist: %v", err)
	}
	if len(got) != 2 || got[0] != "payments" || got[1] != "platform" {
		t.Fatalf("got %v, want sorted unique [payments platform]", got)
	}
}

func TestNormalizeNamespaceAllowlistRejectsInvalidNamespace(t *testing.T) {
	for _, input := range [][]string{{""}, {"Payments"}, {"payments/blue"}} {
		if _, err := normalizeNamespaceAllowlist(input); err == nil {
			t.Errorf("normalizeNamespaceAllowlist(%q) succeeded, want validation error", input)
		}
	}
}

func TestScanCommandExposesNamespaceAllowlistFlag(t *testing.T) {
	exitCode := 0
	cmd := newScanCmd(&exitCode)
	if flag := cmd.Flags().Lookup("namespace-allowlist"); flag == nil {
		t.Fatal("scan command has no --namespace-allowlist flag")
	}
}

func TestScanCommandExposesReportServingFlags(t *testing.T) {
	exitCode := 0
	cmd := newScanCmd(&exitCode)
	for _, name := range []string{"serve-report", "open-report", "listen", "terminal-output"} {
		if flag := cmd.Flags().Lookup(name); flag == nil {
			t.Errorf("scan command has no --%s flag", name)
		}
	}
}

// TestEffectiveTerminalOutput guards the compact-by-default-when-serving
// contract: an explicit --terminal-output always wins; left unset, the
// flag's own "full" default only flips to "compact" once a local server is
// actually starting (serve=true) — CI/script/--serve-report=never runs
// keep today's full terminal output untouched.
func TestEffectiveTerminalOutput(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		explicit, serve bool
		want            string
	}{
		{name: "unset, not serving: stays full", mode: "full", explicit: false, serve: false, want: "full"},
		{name: "unset, serving: becomes compact", mode: "full", explicit: false, serve: true, want: "compact"},
		{name: "explicit full, serving: stays full", mode: "full", explicit: true, serve: true, want: "full"},
		{name: "explicit silent, not serving: stays silent", mode: "silent", explicit: true, serve: false, want: "silent"},
		{name: "explicit compact, not serving: stays compact", mode: "compact", explicit: true, serve: false, want: "compact"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := effectiveTerminalOutput(tc.mode, tc.explicit, tc.serve); got != tc.want {
				t.Fatalf("effectiveTerminalOutput(%q, %v, %v) = %q, want %q", tc.mode, tc.explicit, tc.serve, got, tc.want)
			}
		})
	}
}

func TestWritePartialScanNoticeFormatsCollectorErrors(t *testing.T) {
	var out bytes.Buffer
	writePartialScanNotice(&out, "manifest", map[string]error{
		"manifest-dir:/tmp/missing": fmt.Errorf("Manifest path not found. Check the path or remove --manifests: /tmp/missing"),
	})

	got := out.String()
	for _, want := range []string{
		"Partial manifest scan — collectors failed:",
		"manifest-dir:/tmp/missing: Manifest path not found. Check the path or remove --manifests: /tmp/missing",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, "map[") {
		t.Errorf("output = %q, should not expose Go map formatting", got)
	}
}

func TestInvalidTerminalOutputFailsBeforeClusterAccess(t *testing.T) {
	exitCode := 0
	cmd := newScanCmd(&exitCode)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--target-version", "1.36", "--terminal-output", "verbose"})
	if err := cmd.Execute(); err == nil {
		t.Error("scan --terminal-output verbose succeeded, want flag validation error")
	}
}

func TestMarkdownOutputStillWritesCustomCanonicalJSON(t *testing.T) {
	customJSON := filepath.Join(t.TempDir(), "custom.json")
	targets := requestedReportTargets("md", customJSON, false)
	if len(targets) != 2 || targets[0].path != customJSON || targets[1].path != "report.md" {
		t.Fatalf("markdown targets = %+v, want custom JSON followed by report.md", targets)
	}

	rpt := findings.NewReport("1.36", "test", "", time.Unix(0, 0).UTC(), nil)
	if err := writeReportFile(targets[0].path, rpt, targets[0].write); err != nil {
		t.Fatalf("writing canonical JSON: %v", err)
	}
	raw, err := os.ReadFile(customJSON)
	if err != nil {
		t.Fatalf("custom JSON was not created: %v", err)
	}
	var decoded findings.Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("custom JSON is not a findings report: %v", err)
	}
}

func TestCreateReportFileRefusesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realTarget := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(realTarget, []byte("do not touch"), 0o644); err != nil {
		t.Fatalf("seeding symlink target: %v", err)
	}
	linkPath := filepath.Join(dir, "report.html")
	if err := os.Symlink(realTarget, linkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	if _, err := createReportFile(linkPath); err == nil {
		t.Fatal("createReportFile succeeded on a symlink target, want a refusal error")
	} else if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error = %q, want it to clearly mention symlinks are refused", err.Error())
	}

	raw, err := os.ReadFile(realTarget)
	if err != nil {
		t.Fatalf("reading symlink target after refused write: %v", err)
	}
	if string(raw) != "do not touch" {
		t.Errorf("symlink target = %q, want it untouched by the refused write", raw)
	}
}

func TestCreateReportFileStillOverwritesRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.html")
	if err := os.WriteFile(path, []byte("stale content"), 0o644); err != nil {
		t.Fatalf("seeding existing regular file: %v", err)
	}

	f, err := createReportFile(path)
	if err != nil {
		t.Fatalf("createReportFile on an existing regular file: %v", err)
	}
	if _, err := f.WriteString("fresh content"); err != nil {
		t.Fatalf("writing: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading rewritten file: %v", err)
	}
	if string(raw) != "fresh content" {
		t.Errorf("content = %q, want the regular file to be overwritten normally", raw)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestCreateReportFileWorksForNewPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "findings.json")
	f, err := createReportFile(path)
	if err != nil {
		t.Fatalf("createReportFile on a non-existing path: %v", err)
	}
	f.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to be created: %v", path, err)
	}
}

func TestRequestedReportTargetsEnsuresHTMLWhenServing(t *testing.T) {
	targets := requestedReportTargets("json", "findings.json", true)
	if len(targets) != 2 || targets[0].path != "findings.json" || targets[1].path != "report.html" {
		t.Fatalf("serving targets = %+v, want findings.json + report.html", targets)
	}
}

func TestShouldServeReport(t *testing.T) {
	tests := []struct {
		name                    string
		mode, output            string
		outputExplicit, tty, ci bool
		want                    bool
	}{
		{name: "auto interactive", mode: "auto", output: "json", tty: true, want: true},
		{name: "auto CI", mode: "auto", output: "all", tty: true, ci: true, want: false},
		{name: "auto redirected", mode: "auto", output: "all", want: false},
		{name: "explicit JSON", mode: "auto", output: "json", outputExplicit: true, tty: true, want: false},
		{name: "always redirected CI", mode: "always", output: "json", outputExplicit: true, ci: true, want: true},
		{name: "never interactive", mode: "never", output: "all", tty: true, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldServeReport(tc.mode, tc.output, tc.outputExplicit, tc.tty, tc.ci); got != tc.want {
				t.Fatalf("shouldServeReport() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInteractiveDefaultWritesAllFormats(t *testing.T) {
	if got := effectiveScanOutput("json", false, true); got != "all" {
		t.Fatalf("effectiveScanOutput() = %q, want all", got)
	}
	if got := effectiveScanOutput("json", true, false); got != "json" {
		t.Fatalf("explicit JSON changed to %q", got)
	}
}

func TestBufferIsNotInteractive(t *testing.T) {
	if writerIsTerminal(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer reported as an interactive terminal")
	}
}

func TestInvalidReportServingFlagsFailBeforeClusterAccess(t *testing.T) {
	for _, args := range [][]string{
		{"--target-version", "1.36", "--serve-report", "sometimes"},
		{"--target-version", "1.36", "--serve-report", "never", "--open-report"},
	} {
		exitCode := 0
		cmd := newScanCmd(&exitCode)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("scan %v succeeded, want flag validation error", args)
		}
	}
}

func TestScanCommandListenDefaultsToFixedPort(t *testing.T) {
	exitCode := 0
	cmd := newScanCmd(&exitCode)
	flag := cmd.Flags().Lookup("listen")
	if flag == nil {
		t.Fatal("scan command has no --listen flag")
	}
	if flag.DefValue != "127.0.0.1:8080" {
		t.Errorf("--listen default = %q, want 127.0.0.1:8080", flag.DefValue)
	}
}

func writeReportServerFixtures(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "report.html"), []byte("<h1>r</h1>"), 0o644); err != nil {
		t.Fatalf("write report.html: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "findings.json"), []byte(`{"findings":[]}`), 0o644); err != nil {
		t.Fatalf("write findings.json: %v", err)
	}
}

func TestServeReports_CompactModeOmitsSeparateConsoleURL(t *testing.T) {
	writeReportServerFixtures(t)

	exitCode := 0
	cmd := newScanCmd(&exitCode)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := serveReports(cmd, "findings.json", ".", "127.0.0.1:0", true, false, "compact", false, false); err != nil {
		t.Fatalf("serveReports: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Open KubePreflight report:") {
		t.Errorf("output = %q, want it to contain the report URL heading", got)
	}
	if strings.Contains(got, "Open Console:") {
		t.Errorf("output = %q, compact mode must not print a separate Console URL", got)
	}
}

func TestServeReports_FullModeAlsoPrintsConsoleURL(t *testing.T) {
	writeReportServerFixtures(t)

	exitCode := 0
	cmd := newScanCmd(&exitCode)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := serveReports(cmd, "findings.json", ".", "127.0.0.1:0", true, false, "full", false, false); err != nil {
		t.Fatalf("serveReports: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "Open KubePreflight report:") {
		t.Errorf("output = %q, want it to contain the report URL heading", got)
	}
	if !strings.Contains(got, "Open Console:") {
		t.Errorf("output = %q, full mode must also print the Console URL", got)
	}
}

// TestServeReports_LoopbackDefaultDoesNotWarn guards that the new
// remote-exposure warning stays silent for the common case — a default
// loopback bind with no --allow-remote-report — matching pre-existing
// behavior exactly.
func TestServeReports_LoopbackDefaultDoesNotWarn(t *testing.T) {
	writeReportServerFixtures(t)

	exitCode := 0
	cmd := newScanCmd(&exitCode)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := serveReports(cmd, "findings.json", ".", "127.0.0.1:0", true, false, "compact", false, false); err != nil {
		t.Fatalf("serveReports: %v", err)
	}
	if got := out.String(); strings.Contains(got, "WARNING") {
		t.Errorf("output = %q, must not warn for a default loopback bind", got)
	}
}

// TestServeReports_AllowRemoteReportPrintsPersistentWarning guards the
// actual fix: validateListenAddress's startup gate is a one-time flag
// check, but the warning that the server is unauthenticated and exposed
// must also appear every time the server actually starts serving,
// printed right alongside the URLs a user is looking at.
func TestServeReports_AllowRemoteReportPrintsPersistentWarning(t *testing.T) {
	writeReportServerFixtures(t)

	exitCode := 0
	cmd := newScanCmd(&exitCode)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := serveReports(cmd, "findings.json", ".", "127.0.0.1:0", true, false, "compact", false, true); err != nil {
		t.Fatalf("serveReports: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "WARNING") {
		t.Errorf("output = %q, want a persistent warning when --allow-remote-report is used", got)
	}
	if !strings.Contains(got, "UNAUTHENTICATED") {
		t.Errorf("output = %q, want the warning to explain the server is unauthenticated", got)
	}
	if !strings.Contains(got, "namespaces") || !strings.Contains(got, "UIDs") {
		t.Errorf("output = %q, want the warning to mention what findings data can include", got)
	}
}

func TestScanCommandExposesProviderFlags(t *testing.T) {
	exitCode := 0
	cmd := newScanCmd(&exitCode)
	for _, name := range []string{"provider", "cluster-name", "resource-group", "subscription-id", "project", "location"} {
		if flag := cmd.Flags().Lookup(name); flag == nil {
			t.Errorf("scan command has no --%s flag", name)
		}
	}
}

func TestScanCommandRejectsArgumentsAndInvalidVersionBeforeClusterAccess(t *testing.T) {
	for _, args := range [][]string{{"unexpected", "--target-version", "1.36"}, {"--target-version", "not-a-version"}} {
		cmd := newScanCmd(new(int))
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("Execute(%v) succeeded, want validation error", args)
		}
	}
}

// TestScanCommand_KubeconfigLoadFailureIsInfraFailureNotWarnings guards
// the P0 fix end-to-end through the real scan RunE path: a kubeconfig
// that can't be loaded means no evidence was ever collected, so it must
// be marked as an infrastructure failure (root.go maps this to exit 4),
// never left as an ordinary error that root.go would otherwise map to
// exit 1 — colliding with 1's documented "warnings only" meaning.
func TestScanCommand_KubeconfigLoadFailureIsInfraFailureNotWarnings(t *testing.T) {
	cmd := newScanCmd(new(int))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--target-version", "1.36", "--kubeconfig", filepath.Join(t.TempDir(), "does-not-exist")})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("scan with a nonexistent kubeconfig succeeded, want a kubeconfig-load error")
	}
	if !isInfraFailure(err) {
		t.Errorf("error = %v, want it marked as an infrastructure failure (exit 4), not an ordinary error (exit 1)", err)
	}
}

// TestScanCommand_ManifestsOnlySkipsClusterAccessEntirely is the exact
// scenario the GitHub Action's manifest-only mode depends on: no
// --kubeconfig is given, no cluster is reachable in this test process at
// all, and the scan must still succeed and produce a real, non-INCOMPLETE
// verdict from the manifest fixture alone.
func TestScanCommand_ManifestsOnlySkipsClusterAccessEntirely(t *testing.T) {
	dir := t.TempDir()
	findingsPath := filepath.Join(dir, "findings.json")
	cmd := newScanCmd(new(int))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--target-version", "1.34",
		"--manifests-only",
		"--manifests", filepath.Join("..", "..", "testdata", "manifest-repo", "raw"),
		"--findings-out", findingsPath,
		"--serve-report", "never",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() = %v, want success -- no kubeconfig should ever be loaded in --manifests-only mode", err)
	}

	data, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Fatalf("reading findings.json: %v", err)
	}
	var rpt findings.Report
	if err := json.Unmarshal(data, &rpt); err != nil {
		t.Fatalf("unmarshaling findings.json: %v", err)
	}
	if rpt.Coverage.Kubernetes.Status != findings.CoverageSkipped {
		t.Errorf("Coverage.Kubernetes.Status = %q, want %q (deliberately not requested, not partial)", rpt.Coverage.Kubernetes.Status, findings.CoverageSkipped)
	}
	if rpt.Coverage.Manifests.Status != findings.CoverageComplete {
		t.Errorf("Coverage.Manifests.Status = %q, want %q", rpt.Coverage.Manifests.Status, findings.CoverageComplete)
	}
	if got := rpt.Result(); got != "BLOCKED" {
		t.Errorf("Result() = %q, want BLOCKED (testdata/manifest-repo/raw/psp.yaml is a removed-API positive fixture)", got)
	}
	if rpt.UpgradeReadiness == nil || rpt.UpgradeReadiness.Verdict != "BLOCKED" {
		t.Errorf("UpgradeReadiness = %+v, want a BLOCKED verdict matching Result()", rpt.UpgradeReadiness)
	}
	foundAPI001 := false
	for _, f := range rpt.Findings {
		if f.RuleID == "API-001" {
			foundAPI001 = true
		}
	}
	if !foundAPI001 {
		t.Error("no API-001 finding in a manifests-only scan of a fixture containing a removed-API PodSecurityPolicy")
	}
}

// TestScanCommand_ManifestsOnlyValidatesUpFront guards the three
// --manifests-only validation errors, all returned before any kubeconfig
// load is attempted.
func TestScanCommand_ManifestsOnlyValidatesUpFront(t *testing.T) {
	for name, args := range map[string][]string{
		"requires manifests or helm-chart": {"--target-version", "1.34", "--manifests-only"},
		"rejects --provider":               {"--target-version", "1.34", "--manifests-only", "--manifests", "./testdata", "--provider", "eks", "--cluster-name", "x"},
		"rejects --kubeconfig":             {"--target-version", "1.34", "--manifests-only", "--manifests", "./testdata", "--kubeconfig", "/tmp/does-not-matter"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := newScanCmd(new(int))
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("Execute(%v) succeeded, want a validation error", args)
			}
		})
	}
}

func TestValidateListenAddressRequiresRemoteAcknowledgement(t *testing.T) {
	if err := validateListenAddress("127.0.0.1:8080", false); err != nil {
		t.Fatalf("loopback: %v", err)
	}
	if err := validateListenAddress("0.0.0.0:8080", false); err == nil {
		t.Fatal("remote listen succeeded without acknowledgement")
	}
	if err := validateListenAddress("0.0.0.0:8080", true); err != nil {
		t.Fatalf("acknowledged remote listen: %v", err)
	}
}

func TestRequestedReportTargetsInDir(t *testing.T) {
	targets := requestedReportTargetsInDir("all", filepath.Join("out", "findings.json"), false, "out")
	want := []string{filepath.Join("out", "findings.json"), filepath.Join("out", "report.md"), filepath.Join("out", "report.html")}
	for i := range want {
		if targets[i].path != want[i] {
			t.Fatalf("target %d = %q, want %q", i, targets[i].path, want[i])
		}
	}
}

// TestScanCommandProviderValidationFailsBeforeClusterAccess guards three
// things at once, all before any kubeconfig/cluster access is attempted:
// an unsupported --provider value is rejected; aks/gke with missing
// provider-specific required flags are rejected; and aks/gke with every
// required flag present are still rejected, because enrichment collection
// for those providers isn't implemented yet (Phase 1 never reaches
// cluster/cloud work for them).
func TestScanCommandProviderValidationFailsBeforeClusterAccess(t *testing.T) {
	for _, args := range [][]string{
		{"--target-version", "1.36", "--provider", "gcp"},
		{"--target-version", "1.36", "--provider", "aks", "--cluster-name", "x"},                                        // missing --resource-group
		{"--target-version", "1.36", "--provider", "aks", "--resource-group", "rg"},                                     // missing --cluster-name
		{"--target-version", "1.36", "--provider", "gke", "--cluster-name", "x"},                                        // missing --project/--location
		{"--target-version", "1.36", "--provider", "gke", "--cluster-name", "x", "--project", "p"},                      // missing --location
		{"--target-version", "1.36", "--provider", "aks", "--cluster-name", "x", "--resource-group", "rg"},              // valid flags, but not implemented yet
		{"--target-version", "1.36", "--provider", "gke", "--cluster-name", "x", "--project", "p", "--location", "us1"}, // valid flags, but not implemented yet
	} {
		exitCode := 0
		cmd := newScanCmd(&exitCode)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("scan %v succeeded, want validation error before cluster access", args)
		}
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
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

	if err := serveReports(cmd, "findings.json", "127.0.0.1:0", true, false, "compact"); err != nil {
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

	if err := serveReports(cmd, "findings.json", "127.0.0.1:0", true, false, "full"); err != nil {
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

func TestScanCommandExposesProviderFlags(t *testing.T) {
	exitCode := 0
	cmd := newScanCmd(&exitCode)
	for _, name := range []string{"provider", "cluster-name", "resource-group", "subscription-id", "project", "location"} {
		if flag := cmd.Flags().Lookup(name); flag == nil {
			t.Errorf("scan command has no --%s flag", name)
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

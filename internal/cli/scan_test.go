package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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
	for _, name := range []string{"serve-report", "open-report", "listen"} {
		if flag := cmd.Flags().Lookup(name); flag == nil {
			t.Errorf("scan command has no --%s flag", name)
		}
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

package cli

import (
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

func TestMarkdownOutputStillWritesCustomCanonicalJSON(t *testing.T) {
	customJSON := filepath.Join(t.TempDir(), "custom.json")
	targets := requestedReportTargets("md", customJSON)
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

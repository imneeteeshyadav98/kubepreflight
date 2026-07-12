package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kubepreflight/internal/comparison"
	"kubepreflight/internal/findings"
)

func writeCompareFixture(t *testing.T, dir, name string, fs []findings.Finding) string {
	t.Helper()
	r := findings.NewReport("1.36", "test", "", time.Now().UTC(), fs)
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

func TestCompareCommand_RequiresBaselineAndCurrent(t *testing.T) {
	for name, args := range map[string][]string{
		"missing baseline": {"--current", "x.json", "--json-out", "out.json"},
		"missing current":  {"--baseline", "x.json", "--json-out", "out.json"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := newCompareCmd()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatalf("Execute(%v) succeeded, want a validation error", args)
			}
		})
	}
}

func TestCompareCommand_RequiresAtLeastOneOutputFlag(t *testing.T) {
	dir := t.TempDir()
	baseline := writeCompareFixture(t, dir, "baseline.json", nil)
	current := writeCompareFixture(t, dir, "current.json", nil)

	cmd := newCompareCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--baseline", baseline, "--current", current})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with neither --json-out nor --markdown-out succeeded, want an error")
	}
}

func TestCompareCommand_MissingFileIsInfraFailure(t *testing.T) {
	dir := t.TempDir()
	current := writeCompareFixture(t, dir, "current.json", nil)

	cmd := newCompareCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--baseline", filepath.Join(dir, "does-not-exist.json"), "--current", current, "--json-out", filepath.Join(dir, "out.json")})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() with a nonexistent --baseline succeeded, want an error")
	}
	if !isInfraFailure(err) {
		t.Errorf("error = %v, want it marked as an infrastructure failure (exit 4) -- a missing file is a filesystem problem, not a document problem", err)
	}
}

func TestCompareCommand_MalformedDocumentIsOrdinaryError(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badPath, []byte("{not valid"), 0o644); err != nil {
		t.Fatalf("writing bad fixture: %v", err)
	}
	current := writeCompareFixture(t, dir, "current.json", nil)

	cmd := newCompareCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--baseline", badPath, "--current", current, "--json-out", filepath.Join(dir, "out.json")})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() with a malformed --baseline succeeded, want an error")
	}
	if isInfraFailure(err) {
		t.Error("a malformed document was marked as an infrastructure failure -- want an ordinary (exit 1) error, matching a bad-flags usage error, not exit 4")
	}
}

func TestCompareCommand_WritesJSONAndMarkdown(t *testing.T) {
	dir := t.TempDir()
	blocker := findings.Finding{
		RuleID: "PDB-001", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
		Message:   "disruption budget exhausted",
		Resources: []findings.ResourceReference{findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "api", "uid-1")},
	}
	blocker.Fingerprint = findings.FingerprintV2("PDB-001", "1.36", "", blocker.Resources[0])

	baseline := writeCompareFixture(t, dir, "baseline.json", nil)
	current := writeCompareFixture(t, dir, "current.json", []findings.Finding{blocker})
	jsonOut := filepath.Join(dir, "comparison.json")
	markdownOut := filepath.Join(dir, "comparison.md")

	cmd := newCompareCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--baseline", baseline, "--current", current, "--json-out", jsonOut, "--markdown-out", markdownOut})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() = %v, want success", err)
	}

	jsonRaw, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("reading --json-out: %v", err)
	}
	var cmp comparison.Comparison
	if err := json.Unmarshal(jsonRaw, &cmp); err != nil {
		t.Fatalf("unmarshaling comparison.json: %v", err)
	}
	if cmp.SchemaVersion != comparison.SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", cmp.SchemaVersion, comparison.SchemaVersion)
	}
	if len(cmp.New) != 1 || cmp.Summary.NewBlockers != 1 {
		t.Errorf("comparison.json = %+v, want 1 new blocker", cmp)
	}
	if cmp.Summary.CurrentVerdict != "BLOCKED" {
		t.Errorf("CurrentVerdict = %q, want BLOCKED", cmp.Summary.CurrentVerdict)
	}

	mdRaw, err := os.ReadFile(markdownOut)
	if err != nil {
		t.Fatalf("reading --markdown-out: %v", err)
	}
	if !bytes.Contains(mdRaw, []byte("PDB-001")) {
		t.Error("comparison.md doesn't mention PDB-001")
	}

	if !bytes.Contains(out.Bytes(), []byte("New: 1")) {
		t.Errorf("stdout = %q, want a New: 1 summary line", out.String())
	}
}

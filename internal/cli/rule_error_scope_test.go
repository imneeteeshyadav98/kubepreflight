package cli

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
)

// failingRule is a minimal rules.Rule used only to deterministically force a
// rule-evaluation error, mirroring internal/rules/execution_test.go's
// unexported fakeErrRule (kept package-private there, so this test defines
// its own local equivalent rather than reaching across package boundaries).
type failingRule struct {
	id  string
	err error
}

func (r failingRule) ID() string { return r.id }

func (r failingRule) Evaluate(sc *rules.ScanContext, targetVersion string) ([]findings.Finding, error) {
	return nil, r.err
}

func simulateRuleErrorReport(registry *rules.Registry, sc *rules.ScanContext, targetVersion, findingsPath string) (*findings.Report, error) {
	fs, ruleExecutions, ruleErr := registry.RunAllWithExecutions(sc, targetVersion)
	_ = ruleErr // production CLI surfaces this through RuleExecutions and INCOMPLETE.
	rpt := findings.NewReportWithUpgradeContext(targetVersion, "", "", findings.UpgradeContext(""), time.Now().UTC(), fs)
	rpt.RuleExecutions = ruleExecutions
	if err := writeReportFile(findingsPath, rpt, report.WriteJSON); err != nil {
		return nil, err
	}
	return rpt, nil
}

func TestRuleErrorWritesIncompleteReport(t *testing.T) {
	ruleErr := errors.New("simulated rule failure")
	registry := rules.NewRegistry()
	registry.Register(failingRule{id: "API-001", err: ruleErr})

	outputDir := t.TempDir()
	findingsPath := outputDir + "/findings.json"

	rpt, err := simulateRuleErrorReport(registry, &rules.ScanContext{}, "1.34", findingsPath)
	if err != nil {
		t.Fatalf("simulateRuleErrorReport: %v", err)
	}

	entries, readErr := os.ReadDir(outputDir)
	if readErr != nil {
		t.Fatalf("reading output dir: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one report file written on the rule-error path, found %d entries in %s: %v",
			len(entries), outputDir, entries)
	}
	if _, statErr := os.Stat(findingsPath); statErr != nil {
		t.Fatalf("expected %s to exist, stat returned: %v", findingsPath, statErr)
	}
	if rpt.IsComplete() || rpt.Result() != "INCOMPLETE" || rpt.ExitCode() != 3 {
		t.Fatalf("rule-error report completeness/result/exit = %v/%s/%d, want false/INCOMPLETE/3", rpt.IsComplete(), rpt.Result(), rpt.ExitCode())
	}
	var failed *findings.RuleExecutionRecord
	for i := range rpt.RuleExecutions {
		if rpt.RuleExecutions[i].RuleID == "API-001" {
			failed = &rpt.RuleExecutions[i]
			break
		}
	}
	if failed == nil {
		t.Fatal("API-001 missing from RuleExecutions")
	}
	if failed.State != findings.ExecutionFailed || failed.Reason == "" {
		t.Fatalf("API-001 execution = %+v, want failed with a reason", *failed)
	}
}

func TestRuleErrorReportWriteFailureReturnsInfraExit(t *testing.T) {
	ruleErr := errors.New("simulated rule failure")
	registry := rules.NewRegistry()
	registry.Register(failingRule{id: "API-001", err: ruleErr})

	outputDir := t.TempDir()
	rpt, err := simulateRuleErrorReport(registry, &rules.ScanContext{}, "1.34", outputDir+"/findings.json")
	if err != nil {
		t.Fatalf("simulateRuleErrorReport: %v", err)
	}
	if rpt.ExitCode() != 3 {
		t.Fatalf("rule-error report exit = %d, want 3 before artifact-write failure", rpt.ExitCode())
	}

	writeErr := writeReportFile(outputDir, rpt, report.WriteJSON)
	if writeErr == nil {
		t.Fatal("writeReportFile(directory) succeeded, want failure")
	}
	if got := exitCodeForError(infraFailure(writeErr), rpt.ExitCode()); got != 4 {
		t.Fatalf("exitCodeForError(write failure, report exit 3) = %d, want 4", got)
	}
}

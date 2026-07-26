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

// simulateRuleErrorAbort is a byte-for-byte mirror of scan.go's (and
// plan.go's) own control flow around Registry.RunAllWithExecutions:
//
//	fs, ruleExecutions, err := registry.RunAllWithExecutions(sc, targetVersion)
//	if err != nil {
//	    return fmt.Errorf("running rules: %w", err)
//	}
//	... findings.NewReportWithUpgradeContext(...) ...
//	... writeReportFile(...) ...
//
// See internal/cli/scan.go:280-283 and internal/cli/plan.go:270-273 for the
// real code this mirrors. There is no test seam in scan.go/plan.go today to
// inject a custom Registry into the real cobra RunE closures (their
// registries are always rules.NewDefaultRegistry()/NewManifestsOnlyRegistry()
// internally), so this test protects the *shape* of that control flow via a
// deliberate, documented copy rather than a full end-to-end command
// invocation. If scan.go/plan.go's real sequence around
// RunAllWithExecutions ever changes, this mirror -- and the file:line
// references above -- must be updated in the same change, or this test
// stops proving anything about production behavior.
func simulateRuleErrorAbort(registry *rules.Registry, sc *rules.ScanContext, targetVersion, findingsPath string) error {
	fs, ruleExecutions, err := registry.RunAllWithExecutions(sc, targetVersion)
	if err != nil {
		return err
	}
	rpt := findings.NewReportWithUpgradeContext(targetVersion, "", "", findings.UpgradeContext(""), time.Now().UTC(), fs)
	rpt.RuleExecutions = ruleExecutions
	return writeReportFile(findingsPath, rpt, report.WriteJSON)
}

// TestRuleErrorAbortsBeforeAnyReportIsWritten protects a documented,
// intentional scope boundary of the RuleExecutionRecord feature (PR 1 of
// docs/roadmap/v1.3.0-scope-audit.md's approved 8-PR sequence): when
// Registry.RunAllWithExecutions returns a non-nil error, RunAllWithExecutions
// itself has already built a real findings.RuleExecutionRecord with
// State: findings.ExecutionFailed for the failing rule (see
// internal/rules/execution_test.go's
// TestRunAllWithExecutions_FirstErrorMarkedFailed_SubsequentErrorSwallowed)
// -- but scan.go and plan.go both return immediately on that error, before
// findings.NewReportWithUpgradeContext is ever called and before any report
// file is written. That means ExecutionFailed is not currently observable
// in any user-facing JSON/Markdown/HTML/Terminal/Console output.
//
// This is intentional today, not a bug: PR 1's scope is the
// RuleExecutionRecord model and registry plumbing, not partial-report
// semantics for a rule-evaluation error, which is a materially larger,
// separately-designed change (see rule.go's RunAllWithExecutions doc
// comment). This test exists so that if a future change makes scan.go or
// plan.go emit a partial report on this path, it does so deliberately --
// this test will fail and force that change to be reviewed as the
// intentional behavior change it would be, not an accidental side effect.
func TestRuleErrorAbortsBeforeAnyReportIsWritten(t *testing.T) {
	ruleErr := errors.New("simulated rule failure")
	registry := rules.NewRegistry()
	registry.Register(failingRule{id: "API-001", err: ruleErr})

	outputDir := t.TempDir()
	findingsPath := outputDir + "/findings.json"

	err := simulateRuleErrorAbort(registry, &rules.ScanContext{}, "1.34", findingsPath)
	if err == nil {
		t.Fatal("expected an error when a rule fails, got nil")
	}
	if !errors.Is(err, ruleErr) {
		t.Fatalf("error = %v, want it to wrap the rule's own error", err)
	}

	entries, readErr := os.ReadDir(outputDir)
	if readErr != nil {
		t.Fatalf("reading output dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no report files written on the rule-error path, found %d entries in %s: %v",
			len(entries), outputDir, entries)
	}
	if _, statErr := os.Stat(findingsPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected %s to not exist, stat returned: %v", findingsPath, statErr)
	}
}

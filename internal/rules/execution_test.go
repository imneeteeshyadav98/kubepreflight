package rules

import (
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// recordsByRuleID indexes RunAllWithExecutions' output for convenient
// per-rule assertions in the tests below.
func recordsByRuleID(records []findings.RuleExecutionRecord) map[string]findings.RuleExecutionRecord {
	out := make(map[string]findings.RuleExecutionRecord, len(records))
	for _, rec := range records {
		out[rec.RuleID] = rec
	}
	return out
}

// TestRunAllWithExecutions_DefaultRegistry_CleanRun_AllEvaluated is the
// first required acceptance test: a clean, error-free run of the full
// default registry must produce Applicable/Evaluated for all 31 rules.
func TestRunAllWithExecutions_DefaultRegistry_CleanRun_AllEvaluated(t *testing.T) {
	registry := NewDefaultRegistry()
	sc := &ScanContext{K8s: &k8s.Snapshot{}}

	_, records, err := registry.RunAllWithExecutions(sc, "1.34")
	if err != nil {
		t.Fatalf("RunAllWithExecutions() error = %v, want nil for a clean empty-snapshot run", err)
	}

	all := AllRuleIDs()
	if len(records) != len(all) {
		t.Fatalf("got %d records, want %d (one per AllRuleIDs())", len(records), len(all))
	}
	byID := recordsByRuleID(records)
	for _, ruleID := range all {
		rec, ok := byID[ruleID]
		if !ok {
			t.Errorf("rule %s missing from RuleExecutions", ruleID)
			continue
		}
		if rec.Applicability != findings.ApplicabilityApplicable || rec.State != findings.ExecutionEvaluated {
			t.Errorf("rule %s = %+v, want Applicable/Evaluated on a clean default-registry run", ruleID, rec)
		}
		if rec.Reason != "" {
			t.Errorf("rule %s Reason = %q, want empty for a clean Evaluated run", ruleID, rec.Reason)
		}
	}
}

// TestRunAllWithExecutions_ManifestsOnlyRegistry_ExcludedRulesMarkedNotApplicable
// is the second required acceptance test: the manifests-only registry must
// mark API-001/API-002 Applicable/Evaluated and the other 29 rules
// NotApplicable/NotEvaluated, since NewManifestsOnlyRegistry only ever
// registers API-001/API-002 (defaults.go).
func TestRunAllWithExecutions_ManifestsOnlyRegistry_ExcludedRulesMarkedNotApplicable(t *testing.T) {
	registry := NewManifestsOnlyRegistry()
	sc := &ScanContext{}

	_, records, err := registry.RunAllWithExecutions(sc, "1.34")
	if err != nil {
		t.Fatalf("RunAllWithExecutions() error = %v, want nil", err)
	}

	all := AllRuleIDs()
	if len(records) != len(all) {
		t.Fatalf("got %d records, want %d (one per AllRuleIDs())", len(records), len(all))
	}

	byID := recordsByRuleID(records)
	registered := map[string]bool{"API-001": true, "API-002": true}

	var notApplicableCount int
	for _, ruleID := range all {
		rec, ok := byID[ruleID]
		if !ok {
			t.Errorf("rule %s missing from RuleExecutions", ruleID)
			continue
		}
		if registered[ruleID] {
			if rec.Applicability != findings.ApplicabilityApplicable || rec.State != findings.ExecutionEvaluated {
				t.Errorf("rule %s = %+v, want Applicable/Evaluated (registered in NewManifestsOnlyRegistry)", ruleID, rec)
			}
			continue
		}
		notApplicableCount++
		if rec.Applicability != findings.ApplicabilityNotApplicable || rec.State != findings.ExecutionNotEvaluated {
			t.Errorf("rule %s = %+v, want NotApplicable/NotEvaluated (excluded from NewManifestsOnlyRegistry)", ruleID, rec)
		}
		if rec.Reason == "" {
			t.Errorf("rule %s has no Reason explaining why it's not evaluated in this scan mode", ruleID)
		}
	}
	if notApplicableCount != 29 {
		t.Fatalf("got %d not-applicable rules, want 29 (31 total minus API-001/API-002)", notApplicableCount)
	}
}

// TestRunAllWithExecutions_RegistryCompleteness mirrors
// TestEveryRegisteredRuleHasAnExplicitCategory's exact pattern
// (internal/findings/upgrade_readiness_registry_test.go): every rule ID
// AllRuleIDs() knows about must appear exactly once in RuleExecutions, for
// every registry variant -- so a future 32nd rule, or a registry that
// forgets to account for one, fails a test instead of silently omitting a
// rule from the report.
func TestRunAllWithExecutions_RegistryCompleteness(t *testing.T) {
	all := AllRuleIDs()
	wantSet := make(map[string]bool, len(all))
	for _, id := range all {
		wantSet[id] = true
	}

	for name, tc := range map[string]struct {
		registry *Registry
		sc       *ScanContext
	}{
		"default":        {registry: NewDefaultRegistry(), sc: &ScanContext{K8s: &k8s.Snapshot{}}},
		"manifests-only": {registry: NewManifestsOnlyRegistry(), sc: &ScanContext{}},
	} {
		t.Run(name, func(t *testing.T) {
			_, records, err := tc.registry.RunAllWithExecutions(tc.sc, "1.34")
			if err != nil {
				t.Fatalf("RunAllWithExecutions() error = %v", err)
			}
			seen := make(map[string]int, len(records))
			for _, rec := range records {
				seen[rec.RuleID]++
			}
			for id := range wantSet {
				if seen[id] == 0 {
					t.Errorf("rule %s is missing from RuleExecutions", id)
				} else if seen[id] > 1 {
					t.Errorf("rule %s appears %d times in RuleExecutions, want exactly 1", id, seen[id])
				}
			}
			for id := range seen {
				if !wantSet[id] {
					t.Errorf("RuleExecutions contains %s, which is not in AllRuleIDs()", id)
				}
			}
		})
	}
}

// fakeErrRule is a minimal Rule implementation used only to deterministically
// force an evaluation error under a specific rule ID, without depending on
// any real rule's own error-triggering conditions (which can shift over
// time as rules evolve). It satisfies the Rule interface directly.
type fakeErrRule struct {
	id  string
	err error
}

func (f fakeErrRule) ID() string { return f.id }

func (f fakeErrRule) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	return nil, f.err
}

// TestRunAllWithExecutions_FirstErrorMarkedFailed_SubsequentErrorSwallowed
// is the required test confirming a rule returning an error is marked
// Applicable/Failed -- and explicitly documents, rather than hides, the
// known RunAll first-error-only limitation described in rule.go's doc
// comments and docs/roadmap/v1.3.0-scope-audit.md's Decision 5: only the
// FIRST erroring rule (in registration order) is retained by the
// underlying, deliberately-unfixed RunAll/runAll -- a second rule that also
// errors in the same scan has its error fully discarded before
// RunAllWithExecutions ever sees it, so it incorrectly reports State:
// Evaluated instead of Failed. This test asserts BOTH outcomes: the
// first-rule case that correctly works today, and the second-rule case
// that does not, so the limitation stays visible in the test suite rather
// than silently passing.
func TestRunAllWithExecutions_FirstErrorMarkedFailed_SubsequentErrorSwallowed(t *testing.T) {
	err1 := errors.New("rule A failed")
	err2 := errors.New("rule B failed")

	registry := NewRegistry()
	registry.Register(fakeErrRule{id: "API-001", err: err1})
	registry.Register(fakeErrRule{id: "API-002", err: err2})

	_, records, err := registry.RunAllWithExecutions(&ScanContext{}, "1.34")
	if !errors.Is(err, err1) {
		t.Fatalf("RunAllWithExecutions() error = %v, want it to be (or wrap) the first rule's error", err)
	}
	if errors.Is(err, err2) {
		t.Fatalf("RunAllWithExecutions() error unexpectedly also carries the second rule's error -- RunAll's first-error-only behavior should make this impossible")
	}

	byID := recordsByRuleID(records)

	first, ok := byID["API-001"]
	if !ok {
		t.Fatal("API-001 missing from RuleExecutions")
	}
	if first.Applicability != findings.ApplicabilityApplicable || first.State != findings.ExecutionFailed {
		t.Errorf("API-001 = %+v, want Applicable/Failed (the one rule RunAll's firstErr is attributed to)", first)
	}
	if first.Reason == "" {
		t.Error("API-001 Reason is empty, want the sanitized error text")
	}

	// KNOWN, DOCUMENTED LIMITATION: API-002's own error (err2) is silently
	// discarded by the underlying RunAll before RunAllWithExecutions can
	// see it, so it incorrectly reports Evaluated with no Reason instead of
	// Failed. This assertion intentionally pins today's (wrong, but
	// understood and out-of-scope-to-fix-here) behavior so a future change
	// to RunAll's error handling is forced to touch this test deliberately,
	// not accidentally.
	second, ok := byID["API-002"]
	if !ok {
		t.Fatal("API-002 missing from RuleExecutions")
	}
	if second.Applicability != findings.ApplicabilityApplicable || second.State != findings.ExecutionEvaluated {
		t.Errorf("API-002 = %+v, want Applicable/Evaluated -- documents RunAll's first-error-only limitation (see rule.go RunAll doc comment); this is a known gap, not a passing assertion of correct behavior", second)
	}

	// Every rule ID outside this synthetic 2-rule registry must still be
	// accounted for as not_applicable/not_evaluated.
	var notApplicableCount int
	for _, ruleID := range AllRuleIDs() {
		if ruleID == "API-001" || ruleID == "API-002" {
			continue
		}
		rec, ok := byID[ruleID]
		if !ok {
			t.Errorf("rule %s missing from RuleExecutions", ruleID)
			continue
		}
		notApplicableCount++
		if rec.Applicability != findings.ApplicabilityNotApplicable || rec.State != findings.ExecutionNotEvaluated {
			t.Errorf("rule %s = %+v, want NotApplicable/NotEvaluated (not registered in this synthetic registry)", ruleID, rec)
		}
	}
	if want := len(AllRuleIDs()) - 2; notApplicableCount != want {
		t.Fatalf("got %d not-applicable rules, want %d", notApplicableCount, want)
	}
}

// TestRunAllWithExecutions_InsufficientEvidence_SingleKey confirms one of
// the 7 rules with an explicit collector-Errors-map skip (PDB-001) is
// marked insufficient_evidence, not evaluated, when the specific collector
// key it depends on ("poddisruptionbudgets") errored -- i.e. when PDB-001
// itself (pdb001.go:33-35) would have taken its silent skip branch rather
// than its ran-and-found-nothing branch.
func TestRunAllWithExecutions_InsufficientEvidence_SingleKey(t *testing.T) {
	registry := NewRegistry()
	registry.Register(PDB001{})

	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{"poddisruptionbudgets": errors.New("list failed: connection refused")},
	}}

	fs, records, err := registry.RunAllWithExecutions(sc, "1.34")
	if err != nil {
		t.Fatalf("RunAllWithExecutions() error = %v, want nil", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 -- PDB-001 should have taken its skip-on-missing-evidence branch", len(fs))
	}

	byID := recordsByRuleID(records)
	rec, ok := byID["PDB-001"]
	if !ok {
		t.Fatal("PDB-001 missing from RuleExecutions")
	}
	if rec.Applicability != findings.ApplicabilityApplicable {
		t.Errorf("PDB-001 Applicability = %q, want applicable", rec.Applicability)
	}
	if rec.State != findings.ExecutionInsufficientEvidence {
		t.Errorf("PDB-001 State = %q, want insufficient_evidence", rec.State)
	}
	if rec.Reason == "" || !strings.Contains(rec.Reason, "poddisruptionbudgets") {
		t.Errorf("PDB-001 Reason = %q, want it to name the missing collector key", rec.Reason)
	}
}

// TestRunAllWithExecutions_InsufficientEvidence_MultipleKeysSorted exercises
// PDB-002's two-key skip (pdb002.go:29-34, "poddisruptionbudgets" AND
// "pods") to confirm insufficientEvidenceReason reports every missing key,
// deterministically ordered, not just the first one it happens to check.
func TestRunAllWithExecutions_InsufficientEvidence_MultipleKeysSorted(t *testing.T) {
	registry := NewRegistry()
	registry.Register(PDB002{})

	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{
			"poddisruptionbudgets": errors.New("boom"),
			"pods":                 errors.New("boom"),
		},
	}}

	_, records, err := registry.RunAllWithExecutions(sc, "1.34")
	if err != nil {
		t.Fatalf("RunAllWithExecutions() error = %v, want nil", err)
	}
	byID := recordsByRuleID(records)
	rec, ok := byID["PDB-002"]
	if !ok {
		t.Fatal("PDB-002 missing from RuleExecutions")
	}
	if rec.State != findings.ExecutionInsufficientEvidence {
		t.Fatalf("PDB-002 State = %q, want insufficient_evidence", rec.State)
	}
	wantKeys := []string{"pods", "poddisruptionbudgets"}
	sort.Strings(wantKeys)
	for _, key := range wantKeys {
		if !strings.Contains(rec.Reason, key) {
			t.Errorf("PDB-002 Reason = %q, want it to mention %q", rec.Reason, key)
		}
	}
}

// TestRunAllWithExecutions_ADDON002_NotMarkedInsufficientEvidence confirms
// ADDON-002 is deliberately excluded from the insufficient_evidence
// heuristic (see ruleErrorsMapKeys' doc comment in execution.go): unlike
// the 6 silent-skip rules, ADDON-002 turns an unverifiable add-on into a
// visible Warning finding, so it correctly remains Evaluated even when its
// own underlying AWS Errors key is populated.
func TestRunAllWithExecutions_ADDON002_NotMarkedInsufficientEvidence(t *testing.T) {
	if _, tracked := ruleErrorsMapKeys["ADDON-002"]; tracked {
		t.Fatal("ADDON-002 must not be in ruleErrorsMapKeys -- it has its own distinct visible-finding mechanism, not a silent skip")
	}
}

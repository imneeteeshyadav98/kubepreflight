package comparison

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/v1compat"
)

// legacyRuleExecutionsFinding builds a minimal, valid finding for the
// rule-execution normalization tests below -- deliberately reusing the same
// shape cmpFinding (compare_test.go) already establishes for this package's
// existing tests, so these new tests don't invent a second finding-building
// convention.
func legacyRuleExecutionsFinding(ruleID string) findings.Finding {
	ref := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "api", "uid-1")
	f := findings.Finding{
		RuleID:      ruleID,
		Severity:    findings.SeverityWarning,
		Confidence:  findings.TierStaticCertain,
		Message:     "legacy normalization test finding",
		Resources:   []findings.ResourceReference{ref},
		Fingerprint: findings.FingerprintV2(ruleID, "1.36", "", ref),
	}
	return findings.AssignPriority(f)
}

// ruleExecutionRecordByID finds the record for ruleID, failing the test if
// it isn't present exactly once.
func ruleExecutionRecordByID(t *testing.T, records []findings.RuleExecutionRecord, ruleID string) findings.RuleExecutionRecord {
	t.Helper()
	var found []findings.RuleExecutionRecord
	for _, rec := range records {
		if rec.RuleID == ruleID {
			found = append(found, rec)
		}
	}
	if len(found) != 1 {
		t.Fatalf("found %d RuleExecutionRecord(s) for %s, want exactly 1 (records: %+v)", len(found), ruleID, records)
	}
	return found[0]
}

// Test 1: a native report -- one that already has real RuleExecutions
// entries and RuleExecutionsNormalized unset -- passes through the
// normalizer completely unchanged.
func TestNormalizeRuleExecutions_NativeReportUnchanged(t *testing.T) {
	native := []findings.RuleExecutionRecord{
		{RuleID: "API-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "WH-005", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
	}

	r := findings.NewReport("1.36", "test-cluster", "", time.Now().UTC(), nil)
	r.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete}})
	r.RuleExecutions = append([]findings.RuleExecutionRecord{}, native...)

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}
	if got.RuleExecutionsNormalized {
		t.Error("RuleExecutionsNormalized = true for a native report, want false")
	}
	if !reflect.DeepEqual(got.RuleExecutions, native) {
		t.Errorf("RuleExecutions = %+v, want unchanged %+v", got.RuleExecutions, native)
	}
}

// Test 2: a legacy report (no ruleExecutions field at all) gets exactly one
// RuleExecutionRecord per current rule ID -- asserting the count, not just
// non-emptiness.
func TestNormalizeRuleExecutions_LegacyReportGetsOneRecordPerRuleID(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": "1.0",
		"targetVersion": "1.36",
		"scannedAt": "2025-01-01T00:00:00Z",
		"findings": [],
		"summary": {"blockers": 0, "warnings": 0, "infos": 0},
		"coverage": {"kubernetes": {"status": "complete"}, "aws": {"status": "skipped"}, "manifests": {"status": "skipped"}}
	}`)

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}
	if !r.RuleExecutionsNormalized {
		t.Error("RuleExecutionsNormalized = false for a legacy report, want true")
	}
	want := len(v1compat.ExpectedRuleIDs())
	if len(r.RuleExecutions) != want {
		t.Fatalf("len(RuleExecutions) = %d, want exactly %d (one per current rule ID)", len(r.RuleExecutions), want)
	}
}

// Test 3: a legacy report containing a WH-005 finding marks WH-005
// applicable/evaluated, while an unrelated rule ID with no finding is marked
// applicable/not_evaluated.
func TestNormalizeRuleExecutions_FindingPresentMeansEvaluated(t *testing.T) {
	f := legacyRuleExecutionsFinding("WH-005")
	raw, err := json.Marshal(findingsOnlyLegacyReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}

	wh005 := ruleExecutionRecordByID(t, r.RuleExecutions, "WH-005")
	if wh005.Applicability != findings.ApplicabilityApplicable || wh005.State != findings.ExecutionEvaluated {
		t.Errorf("WH-005 record = %+v, want applicable/evaluated", wh005)
	}
	if wh005.Reason == "" {
		t.Error("WH-005 record has no Reason explaining the backfill")
	}

	unrelated := ruleExecutionRecordByID(t, r.RuleExecutions, "API-001")
	if unrelated.Applicability != findings.ApplicabilityApplicable || unrelated.State != findings.ExecutionNotEvaluated {
		t.Errorf("API-001 record = %+v, want applicable/not_evaluated (no finding present)", unrelated)
	}
}

// Test 4 -- THE CRITICAL SAFETY-INVARIANT TEST.
//
// A legacy report with ZERO findings must never be read as "every rule
// evaluated cleanly." Every single rule ID in the universe must come back
// not_evaluated; none may come back evaluated. This is the exact failure
// mode the task's core safety rule forbids: interpreting the ABSENCE of a
// finding as evidence of a clean pass. If normalizeRuleExecutions is ever
// changed to default new/unknown rule IDs to State: evaluated (e.g. an
// incorrect refactor that flips the zero-value branch), this test fails.
func TestNormalizeRuleExecutions_ZeroFindings_NoneMarkedEvaluated(t *testing.T) {
	raw, err := json.Marshal(findingsOnlyLegacyReport(nil))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}

	if len(r.RuleExecutions) != len(v1compat.ExpectedRuleIDs()) {
		t.Fatalf("len(RuleExecutions) = %d, want %d", len(r.RuleExecutions), len(v1compat.ExpectedRuleIDs()))
	}
	for _, rec := range r.RuleExecutions {
		if rec.State == findings.ExecutionEvaluated {
			t.Errorf("rule %s marked evaluated with zero findings in the legacy report -- absence of a finding must never be read as a clean evaluation", rec.RuleID)
		}
		if rec.State != findings.ExecutionNotEvaluated {
			t.Errorf("rule %s State = %q, want %q", rec.RuleID, rec.State, findings.ExecutionNotEvaluated)
		}
		if rec.Applicability != findings.ApplicabilityApplicable {
			t.Errorf("rule %s Applicability = %q, want %q", rec.RuleID, rec.Applicability, findings.ApplicabilityApplicable)
		}
	}
}

// Test 5: a legacy report containing a finding for an unknown/historical
// rule ID (one no longer in v1compat.ExpectedRuleIDs()/rules.AllRuleIDs())
// leaves that finding untouched in Report.Findings, and does NOT expand or
// otherwise corrupt the canonical RuleExecutions array.
func TestNormalizeRuleExecutions_UnknownRuleIDFindingPreservedNotAddedToExecutions(t *testing.T) {
	f := legacyRuleExecutionsFinding("LEGACY-999")
	raw, err := json.Marshal(findingsOnlyLegacyReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}

	if len(r.Findings) != 1 || r.Findings[0].RuleID != "LEGACY-999" {
		t.Fatalf("Findings = %+v, want the unknown-rule-ID finding preserved untouched", r.Findings)
	}
	if len(r.RuleExecutions) != len(v1compat.ExpectedRuleIDs()) {
		t.Fatalf("len(RuleExecutions) = %d, want %d (unknown rule ID must not expand the canonical array)", len(r.RuleExecutions), len(v1compat.ExpectedRuleIDs()))
	}
	for _, rec := range r.RuleExecutions {
		if rec.RuleID == "LEGACY-999" {
			t.Fatal("RuleExecutions contains an entry for the unknown rule ID LEGACY-999 -- it must be silently skipped for this bookkeeping")
		}
	}
}

// Test 6: 3 findings all from the same rule ID collapse into exactly ONE
// RuleExecutionRecord, not three.
func TestNormalizeRuleExecutions_DuplicateFindingsSameRuleCollapseToOneRecord(t *testing.T) {
	fs := []findings.Finding{
		legacyRuleExecutionsFinding("WH-005"),
		legacyRuleExecutionsFinding("WH-005"),
		legacyRuleExecutionsFinding("WH-005"),
	}
	raw, err := json.Marshal(findingsOnlyLegacyReport(fs))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}

	var count int
	for _, rec := range r.RuleExecutions {
		if rec.RuleID == "WH-005" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("found %d RuleExecutionRecord(s) for WH-005 from 3 duplicate findings, want exactly 1", count)
	}
}

// Test 7: a document with an explicitly empty "ruleExecutions": [] (present,
// not absent) is treated identically to a wholly-absent field -- backfilled,
// not passed through as a bare empty array. This is the documented decision
// for the empty-array-vs-absent ambiguity: Go's JSON decoder does leave a
// distinguishable signal (nil vs. non-nil-empty slice) but there is no
// further evidence (no build/tool-version stamp on Report) to tell "a
// PR-1-native producer genuinely emitted zero entries" apart from "this
// document predates the field", so both collapse to the same conservative
// backfill path.
func TestNormalizeRuleExecutions_ExplicitEmptyArrayIsBackfilled(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": "1.0",
		"targetVersion": "1.36",
		"scannedAt": "2025-01-01T00:00:00Z",
		"findings": [],
		"summary": {"blockers": 0, "warnings": 0, "infos": 0},
		"coverage": {"kubernetes": {"status": "complete"}, "aws": {"status": "skipped"}, "manifests": {"status": "skipped"}},
		"ruleExecutions": []
	}`)

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}
	if !r.RuleExecutionsNormalized {
		t.Error("RuleExecutionsNormalized = false for an explicitly empty ruleExecutions array, want true (treated as needing backfill)")
	}
	if len(r.RuleExecutions) != len(v1compat.ExpectedRuleIDs()) {
		t.Fatalf("len(RuleExecutions) = %d, want %d", len(r.RuleExecutions), len(v1compat.ExpectedRuleIDs()))
	}
}

// Test 8: JSON round-trip. Normalize a legacy report, marshal it, unmarshal
// it back, and confirm RuleExecutionsNormalized/RuleExecutions survive
// intact -- and that re-normalizing the round-tripped report is a no-op.
func TestNormalizeRuleExecutions_JSONRoundTrip(t *testing.T) {
	f := legacyRuleExecutionsFinding("WH-005")
	raw, err := json.Marshal(findingsOnlyLegacyReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	normalized, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}

	roundTripRaw, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("Marshal(normalized): %v", err)
	}
	var roundTripped findings.Report
	if err := json.Unmarshal(roundTripRaw, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !roundTripped.RuleExecutionsNormalized {
		t.Error("RuleExecutionsNormalized lost across JSON round-trip")
	}
	if !reflect.DeepEqual(roundTripped.RuleExecutions, normalized.RuleExecutions) {
		t.Error("RuleExecutions changed across JSON round-trip")
	}

	// Re-normalizing the round-tripped report (already has a full,
	// backfilled RuleExecutions) must not change anything further.
	before := append([]findings.RuleExecutionRecord{}, roundTripped.RuleExecutions...)
	normalizeRuleExecutions(&roundTripped)
	if !reflect.DeepEqual(roundTripped.RuleExecutions, before) {
		t.Error("re-normalizing a round-tripped, already-backfilled report changed RuleExecutions")
	}
	if !roundTripped.RuleExecutionsNormalized {
		t.Error("RuleExecutionsNormalized flipped to false on re-normalization")
	}
}

// Test 9: idempotency. Calling normalizeRuleExecutions a second time on an
// already-normalized report must be a no-op -- same RuleExecutions content,
// same RuleExecutionsNormalized value.
func TestNormalizeRuleExecutions_SecondCallIsNoOp(t *testing.T) {
	f := legacyRuleExecutionsFinding("WH-005")
	r := findingsOnlyLegacyReport([]findings.Finding{f})

	normalizeRuleExecutions(r)
	first := append([]findings.RuleExecutionRecord{}, r.RuleExecutions...)
	firstNormalized := r.RuleExecutionsNormalized

	normalizeRuleExecutions(r)

	if !reflect.DeepEqual(r.RuleExecutions, first) {
		t.Errorf("second normalizeRuleExecutions call changed RuleExecutions: before=%+v after=%+v", first, r.RuleExecutions)
	}
	if r.RuleExecutionsNormalized != firstNormalized {
		t.Errorf("second call changed RuleExecutionsNormalized: before=%v after=%v", firstNormalized, r.RuleExecutionsNormalized)
	}
}

// Test 10: normalization must not alter Findings, Summary, UpgradeReadiness,
// or APICompatibility -- only RuleExecutions/RuleExecutionsNormalized may
// differ. Built from a fully-native report (findings.NewReport already
// computes Findings/Summary/UpgradeReadiness/APICompatibility; it never
// touches RuleExecutions), so this isolates normalizeRuleExecutions' effect
// precisely.
func TestNormalizeRuleExecutions_OtherReportFieldsUntouched(t *testing.T) {
	f := legacyRuleExecutionsFinding("PDB-001")
	r := findings.NewReport("1.36", "test-cluster", "", time.Now().UTC(), []findings.Finding{f})
	r.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete}})

	findingsBefore := append([]findings.Finding{}, r.Findings...)
	summaryBefore := r.Summary
	upgradeReadinessBefore := *r.UpgradeReadiness
	apiCompatBefore := *r.APICompatibility

	normalizeRuleExecutions(r)

	if !reflect.DeepEqual(r.Findings, findingsBefore) {
		t.Errorf("Findings changed by normalization: before=%+v after=%+v", findingsBefore, r.Findings)
	}
	if !reflect.DeepEqual(r.Summary, summaryBefore) {
		t.Errorf("Summary changed by normalization: before=%+v after=%+v", summaryBefore, r.Summary)
	}
	if !reflect.DeepEqual(*r.UpgradeReadiness, upgradeReadinessBefore) {
		t.Errorf("UpgradeReadiness changed by normalization: before=%+v after=%+v", upgradeReadinessBefore, *r.UpgradeReadiness)
	}
	if !reflect.DeepEqual(*r.APICompatibility, apiCompatBefore) {
		t.Errorf("APICompatibility changed by normalization: before=%+v after=%+v", apiCompatBefore, *r.APICompatibility)
	}
	// Sanity check that normalization actually did something, so this test
	// can't pass vacuously.
	if !r.RuleExecutionsNormalized || len(r.RuleExecutions) == 0 {
		t.Fatal("normalizeRuleExecutions did not backfill RuleExecutions -- test setup is wrong")
	}
}

// findingsOnlyLegacyReport builds a *findings.Report shaped like a
// pre-RuleExecutions document: SchemaVersion/TargetVersion/Findings/Summary/
// Coverage populated (via findings.NewReport, so it round-trips through
// LoadAndNormalize's schemaVersion/targetVersion presence check), but with
// RuleExecutions left at its zero value, exactly as a pre-PR-1 build would
// have produced it.
func findingsOnlyLegacyReport(fs []findings.Finding) *findings.Report {
	r := findings.NewReport("1.36", "test-cluster", "", time.Now().UTC(), fs)
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})
	return r
}

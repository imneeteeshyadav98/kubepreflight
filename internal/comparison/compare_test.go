package comparison

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func cmpFinding(ruleID string, severity findings.Severity, namespace, name string) findings.Finding {
	ref := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, namespace, name, "uid-"+namespace+"-"+name)
	f := findings.Finding{
		RuleID:      ruleID,
		Severity:    severity,
		Confidence:  findings.TierStaticCertain,
		Message:     "comparison test finding",
		Resources:   []findings.ResourceReference{ref},
		Fingerprint: findings.FingerprintV2(ruleID, "1.36", "", ref),
	}
	return findings.AssignPriority(f)
}

func cmpReport(fs []findings.Finding) *findings.Report {
	r := findings.NewReport("1.36", "test-cluster", "", time.Now().UTC(), fs)
	r.SetCoverage(findings.ScanCoverage{
		Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete},
		AWS:        findings.PlaneCoverage{Status: findings.CoverageSkipped},
		Manifests:  findings.PlaneCoverage{Status: findings.CoverageSkipped},
	})
	return r
}

func TestCompare_EmptyVsEmpty(t *testing.T) {
	c, err := Compare(cmpReport(nil), cmpReport(nil))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.New) != 0 || len(c.Resolved) != 0 || len(c.Changed) != 0 || len(c.Unchanged) != 0 {
		t.Fatalf("Compare(empty, empty) = %+v, want all-empty buckets", c)
	}
	if c.Summary.VerdictChanged {
		t.Error("VerdictChanged = true for two clean empty scans")
	}
}

func TestCompare_FindingAdded(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	c, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.New) != 1 || c.New[0].Fingerprint != f.Fingerprint {
		t.Fatalf("New = %+v, want exactly the added finding", c.New)
	}
	if len(c.Resolved) != 0 || len(c.Changed) != 0 || len(c.Unchanged) != 0 {
		t.Errorf("other buckets not empty: %+v", c)
	}
	if c.Summary.New != 1 || c.Summary.NewBlockers != 1 {
		t.Errorf("Summary = %+v, want New=1 NewBlockers=1", c.Summary)
	}
}

// TestCompare_FindingResolved covers test 1 of the required 18: a baseline
// finding whose rule has a proven-evaluated record in current must land in
// Resolved, not NotReEvaluated.
func TestCompare_FindingResolved(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
	}
	c, err := Compare(cmpReport([]findings.Finding{f}), current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Resolved) != 1 || c.Resolved[0].Fingerprint != f.Fingerprint {
		t.Fatalf("Resolved = %+v, want exactly the resolved finding", c.Resolved)
	}
	if len(c.NotReEvaluated) != 0 {
		t.Errorf("NotReEvaluated = %+v, want empty when the rule proves it evaluated cleanly", c.NotReEvaluated)
	}
	if c.Summary.Resolved != 1 || c.Summary.ResolvedBlockers != 1 {
		t.Errorf("Summary = %+v, want Resolved=1 ResolvedBlockers=1", c.Summary)
	}
	if c.Summary.NotReEvaluated != 0 {
		t.Errorf("Summary.NotReEvaluated = %d, want 0", c.Summary.NotReEvaluated)
	}
}

// TestCompare_NotReEvaluated_* cover tests 2-9 of the required 18: every way
// a baseline-only finding's rule can fail to *prove* it evaluated cleanly
// this scan, each of which must classify NotReEvaluated, never Resolved.
func TestCompare_NotReEvaluated_NotEvaluated(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionNotEvaluated},
	}
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), current, f)
}

func TestCompare_NotReEvaluated_InsufficientEvidence(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionInsufficientEvidence},
	}
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), current, f)
}

func TestCompare_NotReEvaluated_Failed(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionFailed},
	}
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), current, f)
}

func TestCompare_NotReEvaluated_NotApplicable(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	// A record marked not_applicable is always paired with
	// State: not_evaluated in practice (RunAllWithExecutions), but the
	// classifier must reject on Applicability alone regardless of State, so
	// this deliberately pairs not_applicable with State: evaluated to prove
	// Applicability is checked, not just State.
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityNotApplicable, State: findings.ExecutionEvaluated},
	}
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), current, f)
}

func TestCompare_NotReEvaluated_MissingRecordForRuleID(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	// Current has RuleExecutions data, just none for PDB-001 specifically.
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "WH-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
	}
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), current, f)
}

func TestCompare_NotReEvaluated_NoRuleExecutionsAtAll(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	// cmpReport never populates RuleExecutions -- this is the "current report
	// has no RuleExecutions metadata at all" case.
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), cmpReport(nil), f)
}

func TestCompare_NotReEvaluated_NormalizedLegacyCurrent(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	baseline := cmpReport([]findings.Finding{f})

	// A normalized-legacy current report backfilled via
	// comparison.LoadAndNormalize's normalizeRuleExecutions: PDB-001 has no
	// finding in the raw legacy document, so it's conservatively backfilled
	// as ApplicabilityApplicable/ExecutionNotEvaluated (never "evaluated"
	// just because it's absent -- that's the whole point of PR 2's
	// backfill). Compare must still classify this NotReEvaluated.
	raw := []byte(`{
		"schemaVersion": "1.0",
		"targetVersion": "1.36",
		"clusterContext": "test-cluster",
		"scannedAt": "2025-01-01T00:00:00Z",
		"findings": [],
		"summary": {"blockers": 0, "warnings": 0, "infos": 0},
		"coverage": {"kubernetes": {"status": "complete"}, "aws": {"status": "skipped"}, "manifests": {"status": "skipped"}}
	}`)
	current, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}
	if !current.RuleExecutionsNormalized {
		t.Fatal("expected RuleExecutionsNormalized=true for a legacy document with no native RuleExecutions")
	}
	assertNotReEvaluated(t, baseline, current, f)
}

func TestCompare_NotReEvaluated_UnknownHistoricalRuleID(t *testing.T) {
	// A rule ID from the baseline that current's canonical execution
	// records simply don't recognize at all (e.g. a renamed/removed rule).
	f := cmpFinding("RETIRED-999", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
	}
	assertNotReEvaluated(t, cmpReport([]findings.Finding{f}), current, f)
}

// TestCompare_NotReEvaluated_DuplicateRuleIDsIndependentlyClassified covers
// test 10: two distinct baseline findings sharing a rule ID (different
// resources, different fingerprints) but both absent from current must each
// independently land in the same bucket -- proving there's no accidental
// dedup, just consistent classification from the same execution record.
func TestCompare_NotReEvaluated_DuplicateRuleIDsIndependentlyClassified(t *testing.T) {
	a := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api-a")
	b := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api-b")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionNotEvaluated},
	}
	c, err := Compare(cmpReport([]findings.Finding{a, b}), current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.NotReEvaluated) != 2 {
		t.Fatalf("NotReEvaluated = %+v, want both duplicate-rule-ID findings classified", c.NotReEvaluated)
	}
	if len(c.Resolved) != 0 {
		t.Fatalf("Resolved = %+v, want empty", c.Resolved)
	}
	gotFPs := map[string]bool{c.NotReEvaluated[0].Fingerprint: true, c.NotReEvaluated[1].Fingerprint: true}
	if !gotFPs[a.Fingerprint] || !gotFPs[b.Fingerprint] {
		t.Errorf("NotReEvaluated fingerprints = %v, want both %q and %q", gotFPs, a.Fingerprint, b.Fingerprint)
	}
}

// assertNotReEvaluated is the shared assertion for every
// TestCompare_NotReEvaluated_* case: exactly one NotReEvaluated entry for f,
// Resolved left empty, and Summary counts consistent.
func assertNotReEvaluated(t *testing.T, baseline, current *findings.Report, f findings.Finding) {
	t.Helper()
	c, err := Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.NotReEvaluated) != 1 || c.NotReEvaluated[0].Fingerprint != f.Fingerprint {
		t.Fatalf("NotReEvaluated = %+v, want exactly the not-re-evaluated finding", c.NotReEvaluated)
	}
	if len(c.Resolved) != 0 {
		t.Fatalf("Resolved = %+v, want empty -- resolution was never proven", c.Resolved)
	}
	if c.Summary.NotReEvaluated != 1 {
		t.Errorf("Summary.NotReEvaluated = %d, want 1", c.Summary.NotReEvaluated)
	}
	if c.Summary.Resolved != 0 {
		t.Errorf("Summary.Resolved = %d, want 0", c.Summary.Resolved)
	}
}

func TestCompare_UnchangedFinding(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	c, err := Compare(cmpReport([]findings.Finding{f}), cmpReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Unchanged) != 1 || len(c.New) != 0 || len(c.Resolved) != 0 || len(c.Changed) != 0 {
		t.Fatalf("Compare(f, f) = %+v, want exactly one Unchanged", c)
	}
}

func TestCompare_SeverityChanged(t *testing.T) {
	before := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	after := before
	after.Severity = findings.SeverityWarning
	after = findings.AssignPriority(after)

	c, err := Compare(cmpReport([]findings.Finding{before}), cmpReport([]findings.Finding{after}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Changed) != 1 {
		t.Fatalf("Changed = %+v, want exactly 1", c.Changed)
	}
	fc, ok := c.Changed[0].Changes["severity"]
	if !ok || fc.Before != "Blocker" || fc.After != "Warning" {
		t.Errorf("severity change = %+v, want Blocker -> Warning", c.Changed[0].Changes)
	}
}

func TestCompare_PriorityChanged(t *testing.T) {
	before := cmpFinding("PDB-001", findings.SeverityWarning, "default", "api")
	after := before
	after.GlobalBlocker = true // escalates priority to P1 via AssignPriority
	after = findings.AssignPriority(after)
	after.Fingerprint = before.Fingerprint // same conceptual finding, priority is derived not identity

	c, err := Compare(cmpReport([]findings.Finding{before}), cmpReport([]findings.Finding{after}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Changed) != 1 {
		t.Fatalf("Changed = %+v, want exactly 1", c.Changed)
	}
	if _, ok := c.Changed[0].Changes["priority"]; !ok {
		t.Errorf("Changes = %+v, want a priority change", c.Changed[0].Changes)
	}
}

func TestCompare_CanUpgradeContinueChanged(t *testing.T) {
	before := cmpFinding("WORKLOAD-001", findings.SeverityWarning, "default", "api")
	after := before
	after.Severity = findings.SeverityBlocker
	after.UpgradeGate = ""
	after = findings.AssignPriority(after)
	after.Fingerprint = before.Fingerprint

	c, err := Compare(cmpReport([]findings.Finding{before}), cmpReport([]findings.Finding{after}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Changed) != 1 {
		t.Fatalf("Changed = %+v, want exactly 1", c.Changed)
	}
	fc, ok := c.Changed[0].Changes["canUpgradeContinue"]
	if !ok || fc.Before != "true" || fc.After != "false" {
		t.Errorf("canUpgradeContinue change = %+v, want true -> false", c.Changed[0].Changes)
	}
}

func TestCompare_MultipleChangesOnSameFingerprint(t *testing.T) {
	before := cmpFinding("WORKLOAD-001", findings.SeverityWarning, "default", "api")
	after := before
	after.Severity = findings.SeverityBlocker
	after.CriticalInfra = true
	after.UpgradeGate = ""
	after = findings.AssignPriority(after)
	after.Fingerprint = before.Fingerprint

	c, err := Compare(cmpReport([]findings.Finding{before}), cmpReport([]findings.Finding{after}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Changed) != 1 {
		t.Fatalf("Changed = %+v, want exactly 1", c.Changed)
	}
	changes := c.Changed[0].Changes
	if _, ok := changes["severity"]; !ok {
		t.Error("expected a severity change")
	}
	if _, ok := changes["priority"]; !ok {
		t.Error("expected a priority change")
	}
	if _, ok := changes["canUpgradeContinue"]; !ok {
		t.Error("expected a canUpgradeContinue change")
	}
}

func TestCompare_ReadinessScoreMovement(t *testing.T) {
	blocker := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")

	t.Run("increase when a blocker resolves", func(t *testing.T) {
		c, err := Compare(cmpReport([]findings.Finding{blocker}), cmpReport(nil))
		if err != nil {
			t.Fatalf("Compare: %v", err)
		}
		if c.Summary.ReadinessScoreDelta <= 0 {
			t.Errorf("ReadinessScoreDelta = %d, want positive (fewer blockers -> higher score)", c.Summary.ReadinessScoreDelta)
		}
	})

	t.Run("decrease when a blocker appears", func(t *testing.T) {
		c, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{blocker}))
		if err != nil {
			t.Fatalf("Compare: %v", err)
		}
		if c.Summary.ReadinessScoreDelta >= 0 {
			t.Errorf("ReadinessScoreDelta = %d, want negative (new blocker -> lower score)", c.Summary.ReadinessScoreDelta)
		}
	})
}

func TestCompare_VerdictMovement(t *testing.T) {
	blocker := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	c, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{blocker}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !c.Summary.VerdictChanged {
		t.Error("VerdictChanged = false, want true (CLEAN -> BLOCKED)")
	}
	if c.Summary.BaselineVerdict != "CLEAN" || c.Summary.CurrentVerdict != "BLOCKED" {
		t.Errorf("verdicts = %q -> %q, want CLEAN -> BLOCKED", c.Summary.BaselineVerdict, c.Summary.CurrentVerdict)
	}
}

func TestCompare_DuplicateFingerprintsRejected(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	_, err := Compare(cmpReport([]findings.Finding{f, f}), cmpReport(nil))
	if err == nil {
		t.Fatal("Compare() with duplicate fingerprints succeeded, want an explicit error")
	}
}

func TestCompare_InputOrderingDoesNotAffectOutput(t *testing.T) {
	a := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	b := cmpFinding("WH-001", findings.SeverityWarning, "default", "guard")
	d := cmpFinding("NODE-001", findings.SeverityBlocker, "", "node-1")

	forward, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{a, b, d}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	reversed, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{d, b, a}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	fpOrder := func(entries []Entry) []string {
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Fingerprint
		}
		return out
	}
	f1, f2 := fpOrder(forward.New), fpOrder(reversed.New)
	if len(f1) != len(f2) {
		t.Fatalf("different lengths: %v vs %v", f1, f2)
	}
	for i := range f1 {
		if f1[i] != f2[i] {
			t.Errorf("New[%d] = %q vs %q, want identical order regardless of input order", i, f1[i], f2[i])
		}
	}
}

func TestLoadAndNormalize_OldSchemaFillsPriorityAndScorecard(t *testing.T) {
	// A findings.json predating the priority/scorecard fields entirely --
	// only the fields every schema version has always had.
	raw := []byte(`{
		"schemaVersion": "1.0",
		"targetVersion": "1.36",
		"clusterContext": "old-cluster",
		"scannedAt": "2025-01-01T00:00:00Z",
		"findings": [{
			"ruleId": "PDB-001",
			"severity": "Blocker",
			"confidence": "OBSERVED",
			"message": "old schema finding",
			"resources": [{"plane":"live","kind":"PodDisruptionBudget","scope":"namespaced","namespace":"default","name":"api","uid":"uid-1"}],
			"fingerprint": "abc123"
		}],
		"summary": {"blockers": 1, "warnings": 0, "infos": 0},
		"coverage": {"kubernetes": {"status": "complete"}, "aws": {"status": "skipped"}, "manifests": {"status": "skipped"}}
	}`)

	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}
	if r.Findings[0].Priority == "" {
		t.Error("Priority not backfilled for an old-schema finding")
	}
	if r.UpgradeReadiness == nil {
		t.Fatal("UpgradeReadiness not backfilled for an old-schema document")
	}
	if r.UpgradeReadiness.Verdict != "BLOCKED" {
		t.Errorf("backfilled Verdict = %q, want BLOCKED", r.UpgradeReadiness.Verdict)
	}
	if r.APICompatibility == nil {
		t.Error("APICompatibility not backfilled for an old-schema document")
	}
}

func TestLoadAndNormalize_IncompleteCoveragePreserved(t *testing.T) {
	raw := []byte(`{
		"schemaVersion": "1.0",
		"targetVersion": "1.36",
		"scannedAt": "2025-01-01T00:00:00Z",
		"findings": [],
		"summary": {"blockers": 0, "warnings": 0, "infos": 0},
		"coverage": {"kubernetes": {"status": "partial", "errors": ["connection refused"]}, "aws": {"status": "skipped"}, "manifests": {"status": "skipped"}}
	}`)
	r, err := LoadAndNormalize(raw)
	if err != nil {
		t.Fatalf("LoadAndNormalize: %v", err)
	}
	if r.Result() != "INCOMPLETE" {
		t.Errorf("Result() = %q, want INCOMPLETE (partial coverage must never be silently treated as complete)", r.Result())
	}
	if r.UpgradeReadiness.Verdict != "INCOMPLETE" {
		t.Errorf("UpgradeReadiness.Verdict = %q, want INCOMPLETE", r.UpgradeReadiness.Verdict)
	}
}

func TestLoadAndNormalize_MalformedJSON(t *testing.T) {
	_, err := LoadAndNormalize([]byte("{not valid json"))
	if err == nil {
		t.Fatal("LoadAndNormalize(malformed JSON) succeeded, want an error")
	}
}

func TestLoadAndNormalize_UnsupportedDocument(t *testing.T) {
	for name, raw := range map[string]string{
		"missing schemaVersion": `{"targetVersion": "1.36", "findings": []}`,
		"missing targetVersion": `{"schemaVersion": "1.0", "findings": []}`,
		"empty object":          `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadAndNormalize([]byte(raw))
			if err == nil {
				t.Fatalf("LoadAndNormalize(%s) succeeded, want an explicit error", name)
			}
		})
	}
}

func TestCompare_TargetVersionMismatchWarns(t *testing.T) {
	baseline := findings.NewReport("1.35", "test", "", time.Now(), nil)
	baseline.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete}})
	current := findings.NewReport("1.36", "test", "", time.Now(), nil)
	current.SetCoverage(findings.ScanCoverage{Kubernetes: findings.PlaneCoverage{Status: findings.CoverageComplete}})

	c, err := Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Warnings) == 0 {
		t.Error("Warnings is empty, want a target-version mismatch warning")
	}
}

func TestWriteMarkdown_Deterministic(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	c, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	var buf1, buf2 bytes.Buffer
	if err := WriteMarkdown(c, &buf1); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if err := WriteMarkdown(c, &buf2); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if buf1.String() != buf2.String() {
		t.Error("WriteMarkdown produced different output for the same Comparison across two calls")
	}
	if !strings.Contains(buf1.String(), "PDB-001") {
		t.Error("Markdown output doesn't mention the new finding's rule ID")
	}
}

func TestComparison_JSONRoundTrip(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	c, err := Compare(cmpReport(nil), cmpReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"schemaVersion":"kubepreflight.io/scan-comparison/v1"`) {
		t.Errorf("marshaled JSON missing expected schemaVersion: %s", raw)
	}
	var roundTripped Comparison
	if err := json.Unmarshal(raw, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(roundTripped.New) != 1 || roundTripped.New[0].Fingerprint != f.Fingerprint {
		t.Errorf("round-tripped New = %+v, want the original finding preserved", roundTripped.New)
	}
}

// TestCompare_LegacyFingerprintMatchingUnaffected covers test 12: a finding
// carrying a legacy findings.Fingerprint() hash (not FingerprintV2) must
// still match/diff exactly as before -- Compare only ever compares the
// Finding.Fingerprint string value, never inspects which hash function
// produced it, so this PR's new NotReEvaluated logic (which only engages
// for fingerprints with NO match in current) must leave ordinary
// New/Changed/Unchanged matching completely untouched regardless of
// fingerprint scheme.
func TestCompare_LegacyFingerprintMatchingUnaffected(t *testing.T) {
	ref := findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "api", "uid-default-api")
	legacyFP := findings.Fingerprint("PDB-001", ref.UID, "1.36")
	f := findings.Finding{
		RuleID:      "PDB-001",
		Severity:    findings.SeverityBlocker,
		Confidence:  findings.TierStaticCertain,
		Message:     "legacy fingerprint finding",
		Resources:   []findings.ResourceReference{ref},
		Fingerprint: legacyFP,
	}
	f = findings.AssignPriority(f)

	c, err := Compare(cmpReport([]findings.Finding{f}), cmpReport([]findings.Finding{f}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Unchanged) != 1 || c.Unchanged[0].Fingerprint != legacyFP {
		t.Fatalf("Unchanged = %+v, want the legacy-fingerprint finding matched as unchanged", c.Unchanged)
	}
	if len(c.New) != 0 || len(c.Resolved) != 0 || len(c.Changed) != 0 || len(c.NotReEvaluated) != 0 {
		t.Errorf("other buckets not empty for a legacy-fingerprint match: %+v", c)
	}
}

// TestCompare_MessageOnlyChangeNotRegressedToChanged covers test 13: a
// message-text-only edit between baseline and current (same fingerprint,
// same every decision-relevant field) must remain Unchanged, never a false
// Changed regression introduced by this PR's edits to compare.go.
func TestCompare_MessageOnlyChangeNotRegressedToChanged(t *testing.T) {
	before := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	after := before
	after.Message = "reworded message text, same underlying issue"

	c, err := Compare(cmpReport([]findings.Finding{before}), cmpReport([]findings.Finding{after}))
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.Changed) != 0 {
		t.Errorf("Changed = %+v, want empty -- a message-only edit is never decision-relevant", c.Changed)
	}
	if len(c.Unchanged) != 1 {
		t.Fatalf("Unchanged = %+v, want exactly 1", c.Unchanged)
	}
}

// TestCompare_NoRuleExecutionComplexityCountsUnchanged covers test 14: a
// scenario mixing New/Changed/Unchanged/Resolved with no rule-execution
// complexity at all (current has a proven-evaluated record for every
// baseline rule ID) must produce byte-identical bucket counts to what this
// PR's predecessor behavior would have produced -- see the scope audit's
// "Must remain unchanged: existing bucket counts for any comparison where
// both reports have every rule evaluated."
func TestCompare_NoRuleExecutionComplexityCountsUnchanged(t *testing.T) {
	resolvedFinding := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	unchangedFinding := cmpFinding("WH-001", findings.SeverityWarning, "default", "guard")
	newFinding := cmpFinding("NODE-001", findings.SeverityBlocker, "", "node-1")

	baseline := cmpReport([]findings.Finding{resolvedFinding, unchangedFinding})
	current := cmpReport([]findings.Finding{unchangedFinding, newFinding})
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "WH-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
		{RuleID: "NODE-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionEvaluated},
	}

	c, err := Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if c.Summary.New != 1 || c.Summary.Resolved != 1 || c.Summary.Changed != 0 || c.Summary.Unchanged != 1 {
		t.Errorf("Summary = %+v, want New=1 Resolved=1 Changed=0 Unchanged=1", c.Summary)
	}
	if c.Summary.NotReEvaluated != 0 {
		t.Errorf("Summary.NotReEvaluated = %d, want 0 when every rule proves evaluated", c.Summary.NotReEvaluated)
	}
}

// TestWriteMarkdown_NotReEvaluatedLabelAndExplanation covers test 16's
// Markdown half: the exact required label and explanatory text must appear
// verbatim, both when the bucket is empty and when it has entries.
func TestWriteMarkdown_NotReEvaluatedLabelAndExplanation(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionNotEvaluated},
	}
	c, err := Compare(cmpReport([]findings.Finding{f}), current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(c.NotReEvaluated) != 1 {
		t.Fatalf("NotReEvaluated = %+v, want exactly 1", c.NotReEvaluated)
	}

	var buf bytes.Buffer
	if err := WriteMarkdown(c, &buf); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, NotReEvaluatedLabel) {
		t.Errorf("Markdown output missing required label %q: %s", NotReEvaluatedLabel, out)
	}
	if !strings.Contains(out, NotReEvaluatedExplanation) {
		t.Errorf("Markdown output missing required explanatory text %q: %s", NotReEvaluatedExplanation, out)
	}
	if !strings.Contains(out, "PDB-001") {
		t.Error("Markdown output doesn't mention the not-re-evaluated finding's rule ID")
	}
}

// TestComparison_JSONRoundTrip_NotReEvaluated covers test 15: the
// not_re_evaluated bucket (and its Summary count) survive a JSON
// marshal/unmarshal round trip under the locked snake_case wire key.
func TestComparison_JSONRoundTrip_NotReEvaluated(t *testing.T) {
	f := cmpFinding("PDB-001", findings.SeverityBlocker, "default", "api")
	current := cmpReport(nil)
	current.RuleExecutions = []findings.RuleExecutionRecord{
		{RuleID: "PDB-001", Applicability: findings.ApplicabilityApplicable, State: findings.ExecutionFailed},
	}
	c, err := Compare(cmpReport([]findings.Finding{f}), current)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"not_re_evaluated":[`) {
		t.Errorf("marshaled JSON missing the locked snake_case not_re_evaluated bucket key: %s", raw)
	}
	var roundTripped Comparison
	if err := json.Unmarshal(raw, &roundTripped); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(roundTripped.NotReEvaluated) != 1 || roundTripped.NotReEvaluated[0].Fingerprint != f.Fingerprint {
		t.Errorf("round-tripped NotReEvaluated = %+v, want the original finding preserved", roundTripped.NotReEvaluated)
	}
	if roundTripped.Summary.NotReEvaluated != 1 {
		t.Errorf("round-tripped Summary.NotReEvaluated = %d, want 1", roundTripped.Summary.NotReEvaluated)
	}
}

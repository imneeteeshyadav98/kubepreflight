package rules

import (
	"path/filepath"
	"strings"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

func TestPDB001_Positive_ZeroDisruptionsAllowed(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "PDB-001" {
		t.Errorf("RuleID = %q, want PDB-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Resources[0].Name != "singleton-pdb" || f.Resources[0].Namespace != "payments" {
		t.Errorf("Resources = %+v, want payments/singleton-pdb", f.Resources)
	}
}

func TestPDB001_Negative_DisruptionsAllowedNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb001", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (disruptionsAllowed > 0 must not fire): %+v", len(fs), fs)
	}
}

func TestPDB001_SkipsNoPodsAndStaleStatus(t *testing.T) {
	base := policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "pdb", Namespace: "default", UID: "uid", Generation: 2}}
	noPods := base.DeepCopy()
	noPods.Status.ObservedGeneration = 2
	stale := base.DeepCopy()
	stale.Status.ExpectedPods = 1
	stale.Status.ObservedGeneration = 1
	for _, pdb := range []policyv1.PodDisruptionBudget{*noPods, *stale} {
		fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{PodDisruptionBudgets: []policyv1.PodDisruptionBudget{pdb}, Errors: map[string]error{}}}, "1.34")
		if err != nil || len(fs) != 0 {
			t.Fatalf("Evaluate() = %+v, %v; want no false blocker", fs, err)
		}
	}
}

func TestPDB001_RemediationDetail_DoesNotMislabelExpectedPodsAsReplicas(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "pdb001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (PDB001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	rd := fs[0].RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated")
	}
	if len(rd.Changes) != 1 || rd.Changes[0].Field != "disruptionsAllowed" {
		t.Fatalf("Changes = %+v, expectedPods must not be presented as workload replicas", rd.Changes)
	}
	if rd.SafeFix == nil || rd.Emergency == nil {
		t.Errorf("SafeFix/Emergency = %+v/%+v, want both populated", rd.SafeFix, rd.Emergency)
	}
	if rd.VerifyCommand == "" || rd.ExpectedResult == "" {
		t.Error("VerifyCommand/ExpectedResult must be populated")
	}
}

// TestPDB001_RemediationDetail_PercentageMinAvailableOmitsReplicasChange
// guards that a percentage-based minAvailable doesn't get a fabricated
// "required replicas" number — only the honestly-derivable
// disruptionsAllowed row is shown.
func TestPDB001_RemediationDetail_PercentageMinAvailableOmitsReplicasChange(t *testing.T) {
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "pct-pdb", Namespace: "payments"},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{Type: intstr.String, StrVal: "50%"},
		},
		Status: policyv1.PodDisruptionBudgetStatus{
			DisruptionsAllowed: 0, CurrentHealthy: 2, DesiredHealthy: 2, ExpectedPods: 2,
		},
	}
	f := pdb001Finding(pdb, "1.34")
	rd := f.RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated")
	}
	if len(rd.Changes) != 1 || rd.Changes[0].Field != "disruptionsAllowed" {
		t.Errorf("Changes = %+v, want exactly the disruptionsAllowed row", rd.Changes)
	}
}

func TestPDB001_EmergencyPatchPreservesBudgetFieldAndIncludesRollback(t *testing.T) {
	maxUnavailable := intstr.FromInt(1)
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "payments"},
		Spec:       policyv1.PodDisruptionBudgetSpec{MaxUnavailable: &maxUnavailable},
		Status:     policyv1.PodDisruptionBudgetStatus{ExpectedPods: 3},
	}
	action := pdbEmergencyAction(pdb)
	if action == nil || !strings.Contains(action.Command, "/spec/maxUnavailable") || strings.Contains(action.Command, "/spec/minAvailable") || !strings.Contains(action.Command, "Revert immediately") {
		t.Fatalf("Emergency action = %+v, want maxUnavailable-only patch plus rollback", action)
	}
}

func maxUnavailablePDB(current int, expectedPods int32) policyv1.PodDisruptionBudget {
	value := intstr.FromInt(current)
	return policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "payments"},
		Spec:       policyv1.PodDisruptionBudgetSpec{MaxUnavailable: &value},
		Status:     policyv1.PodDisruptionBudgetStatus{ExpectedPods: expectedPods},
	}
}

// TestPDB001_EmergencyMaxUnavailable_RelaxesRelativeToCurrentValue guards
// the exact regression found in review: the maxUnavailable emergency
// patch previously hardcoded the relaxed value to a bare 1, which could
// be a no-op or actively tighten the budget when the current value was
// already >= 1. The patched value must always be strictly more permissive
// than the current one (the PDB's own expectedPods, its safe full-
// relaxation ceiling), never a blind constant.
func TestPDB001_EmergencyMaxUnavailable_RelaxesRelativeToCurrentValue(t *testing.T) {
	tests := []struct {
		name             string
		current          int
		expectedPods     int32
		wantCopyReady    bool
		wantPatchedValue string
	}{
		{name: "current 0, expectedPods 3 -> relax to 3", current: 0, expectedPods: 3, wantCopyReady: true, wantPatchedValue: "3"},
		{name: "current 1, expectedPods 3 -> relax to 3, never to 1", current: 1, expectedPods: 3, wantCopyReady: true, wantPatchedValue: "3"},
		{name: "current 2, expectedPods 5 -> relax to 5, never to 1", current: 2, expectedPods: 5, wantCopyReady: true, wantPatchedValue: "5"},
		{name: "current 1, expectedPods 1 -> already at ceiling, inspect-first only", current: 1, expectedPods: 1, wantCopyReady: false},
		{name: "current 3, expectedPods 2 -> current already more permissive, inspect-first only", current: 3, expectedPods: 2, wantCopyReady: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			action := pdbEmergencyAction(maxUnavailablePDB(tc.current, tc.expectedPods))
			if action == nil {
				t.Fatal("pdbEmergencyAction() = nil, want an action (safe or inspect-first)")
			}
			// Only the relax (first) command matters here — the revert
			// command legitimately restores the original value, which is
			// correctly "1" whenever the fixture's current value was 1.
			relaxCommand, _, _ := strings.Cut(action.Command, "# Revert")
			if tc.current != 1 && strings.Contains(relaxCommand, `"value":1}`) {
				t.Errorf("relax command = %q, must never contain a hardcoded relax-to-1 patch", relaxCommand)
			}
			if tc.wantCopyReady {
				if !strings.Contains(action.Command, "kubectl patch pdb") || !strings.Contains(relaxCommand, `"value":`+tc.wantPatchedValue+`}`) {
					t.Errorf("Command = %q, want a copy-ready patch to %s", action.Command, tc.wantPatchedValue)
				}
			} else if strings.Contains(action.Command, "kubectl patch pdb") {
				t.Errorf("Command = %q, want inspect-first guidance (no copy-ready patch) when the current value is already at/above the safe ceiling", action.Command)
			}
		})
	}
}

// TestPDB001_EmergencyMaxUnavailable_PercentageIsInspectFirst guards that a
// percentage-based maxUnavailable never gets a copy-ready patch — it can't
// be safely rewritten to a guaranteed-more-permissive absolute value
// without knowing the intended replica count.
func TestPDB001_EmergencyMaxUnavailable_PercentageIsInspectFirst(t *testing.T) {
	pct := intstr.FromString("50%")
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "payments"},
		Spec:       policyv1.PodDisruptionBudgetSpec{MaxUnavailable: &pct},
		Status:     policyv1.PodDisruptionBudgetStatus{ExpectedPods: 10},
	}
	action := pdbEmergencyAction(pdb)
	if action == nil {
		t.Fatal("pdbEmergencyAction() = nil, want inspect-first guidance")
	}
	if strings.Contains(action.Command, "kubectl patch pdb") {
		t.Errorf("Command = %q, want inspect-first guidance (no copy-ready patch) for a percentage-based maxUnavailable", action.Command)
	}
}

// TestPDB001_EmergencyMinAvailable_StillRelaxesToZero guards that the
// pre-existing, already-correct minAvailable path (full relaxation to 0,
// always maximally permissive) is unaffected by the maxUnavailable fix.
func TestPDB001_EmergencyMinAvailable_StillRelaxesToZero(t *testing.T) {
	minAvailable := intstr.FromInt(3)
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "payments"},
		Spec:       policyv1.PodDisruptionBudgetSpec{MinAvailable: &minAvailable},
	}
	action := pdbEmergencyAction(pdb)
	if action == nil || !strings.Contains(action.Command, `"/spec/minAvailable","value":0}`) {
		t.Fatalf("Emergency action = %+v, want a minAvailable:0 patch", action)
	}
}

// TestPDB001_EmergencyMinAvailable_PatchHasTestBeforeReplace guards the
// stale-state safety fix: the emergency patch must include a JSON Patch
// "test" precondition (checking the value observed at scan time) before
// the "replace" op, so the patch fails closed instead of silently
// overwriting a concurrent change if the PDB was modified since the scan.
func TestPDB001_EmergencyMinAvailable_PatchHasTestBeforeReplace(t *testing.T) {
	minAvailable := intstr.FromInt(3)
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "payments"},
		Spec:       policyv1.PodDisruptionBudgetSpec{MinAvailable: &minAvailable},
	}
	action := pdbEmergencyAction(pdb)
	relaxCommand, _, _ := strings.Cut(action.Command, "# Revert")
	testIdx := strings.Index(relaxCommand, `"op":"test"`)
	replaceIdx := strings.Index(relaxCommand, `"op":"replace"`)
	if testIdx == -1 || replaceIdx == -1 || testIdx > replaceIdx {
		t.Fatalf("relax command = %q, want a \"test\" op on /spec/minAvailable before the \"replace\" op", relaxCommand)
	}
	if !strings.Contains(relaxCommand, `"op":"test","path":"/spec/minAvailable","value":3}`) {
		t.Errorf("relax command = %q, want the test op to check the scan-time value (3)", relaxCommand)
	}
	// The revert command must be equally guarded — it should test against
	// the temporary value (0) it expects to be reverting, not blindly
	// reassert the original.
	if !strings.Contains(action.Command, `"op":"test","path":"/spec/minAvailable","value":0},{"op":"replace","path":"/spec/minAvailable","value":3}`) {
		t.Errorf("Command = %q, want the revert op to test against the temporary value (0) before restoring the original (3)", action.Command)
	}
}

// TestPDB001_EmergencyMaxUnavailable_PatchHasTestBeforeReplace mirrors the
// minAvailable case for the maxUnavailable copy-ready-patch path.
func TestPDB001_EmergencyMaxUnavailable_PatchHasTestBeforeReplace(t *testing.T) {
	action := pdbEmergencyAction(maxUnavailablePDB(0, 3))
	if action == nil {
		t.Fatal("pdbEmergencyAction() = nil, want a copy-ready patch")
	}
	relaxCommand, _, _ := strings.Cut(action.Command, "# Revert")
	testIdx := strings.Index(relaxCommand, `"op":"test"`)
	replaceIdx := strings.Index(relaxCommand, `"op":"replace"`)
	if testIdx == -1 || replaceIdx == -1 || testIdx > replaceIdx {
		t.Fatalf("relax command = %q, want a \"test\" op on /spec/maxUnavailable before the \"replace\" op", relaxCommand)
	}
	if !strings.Contains(relaxCommand, `"op":"test","path":"/spec/maxUnavailable","value":0}`) {
		t.Errorf("relax command = %q, want the test op to check the scan-time value (0)", relaxCommand)
	}
	if !strings.Contains(action.Command, `"op":"test","path":"/spec/maxUnavailable","value":3},{"op":"replace","path":"/spec/maxUnavailable","value":0}`) {
		t.Errorf("Command = %q, want the revert op to test against the temporary value (3) before restoring the original (0)", action.Command)
	}
}

// TestPDB001_EmergencyMaxUnavailable_PercentageStillInspectFirstNotJSONError
// guards that the JSON-marshal-error fallback path is distinct from, and
// doesn't interfere with, the pre-existing percentage-is-inspect-first
// behavior (percentage values marshal to JSON just fine as strings — they
// take the inspect-first path because the *relaxation math* isn't safe,
// not because marshaling fails).
func TestPDB001_EmergencyMaxUnavailable_PercentageStillInspectFirstNotJSONError(t *testing.T) {
	pct := intstr.FromString("50%")
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Name: "api-pdb", Namespace: "payments"},
		Spec:       policyv1.PodDisruptionBudgetSpec{MaxUnavailable: &pct},
		Status:     policyv1.PodDisruptionBudgetStatus{ExpectedPods: 10},
	}
	action := pdbEmergencyAction(pdb)
	if action == nil || strings.Contains(action.Command, "kubectl patch pdb") {
		t.Fatalf("Command = %+v, want inspect-first guidance for a percentage-based maxUnavailable", action)
	}
	if !strings.Contains(action.Command, "kubectl get pdb") {
		t.Errorf("Command = %q, want inspect-first guidance to still suggest inspecting the live object", action.Command)
	}
}

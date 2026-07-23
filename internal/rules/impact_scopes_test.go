package rules

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func TestExplicitImpactScopesForNonBlockingOperationalFindings(t *testing.T) {
	cases := []struct {
		name    string
		finding findings.Finding
		want    []findings.ImpactScope
	}{
		{
			name:    "NODE-003",
			finding: impactScopeNODE003Finding(t),
			want: []findings.ImpactScope{
				findings.ImpactScopeWorkerRollout,
				findings.ImpactScopeNodeDrain,
				findings.ImpactScopeWorkloadRestart,
				findings.ImpactScopeFutureMaintenance,
			},
		},
		{
			name:    "DRAIN-001",
			finding: impactScopeDRAIN001Finding(t),
			want: []findings.ImpactScope{
				findings.ImpactScopeWorkerRollout,
				findings.ImpactScopeNodeDrain,
				findings.ImpactScopeWorkloadRestart,
				findings.ImpactScopeCurrentHealth,
			},
		},
		{
			name:    "DRAIN-003",
			finding: impactScopeDRAIN003Finding(t),
			want: []findings.ImpactScope{
				findings.ImpactScopeWorkerRollout,
				findings.ImpactScopeNodeDrain,
				findings.ImpactScopeWorkloadRestart,
			},
		},
		{
			name:    "DRAIN-004",
			finding: impactScopeDRAIN004Finding(t),
			want: []findings.ImpactScope{
				findings.ImpactScopeWorkerRollout,
				findings.ImpactScopeNodeDrain,
				findings.ImpactScopeCurrentHealth,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertImpactScopeMetadataOnly(t, tc.finding, tc.want)
		})
	}
}

func assertImpactScopeMetadataOnly(t *testing.T, f findings.Finding, want []findings.ImpactScope) {
	t.Helper()
	if !reflect.DeepEqual(f.ImpactScopes, want) {
		t.Fatalf("ImpactScopes = %v, want %v", f.ImpactScopes, want)
	}
	seen := map[findings.ImpactScope]bool{}
	for _, scope := range f.ImpactScopes {
		if seen[scope] {
			t.Fatalf("ImpactScopes = %v, duplicate %q", f.ImpactScopes, scope)
		}
		seen[scope] = true
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("finding validation failed with impact scopes: %v", err)
	}

	withoutScopes := f
	withoutScopes.ImpactScopes = nil
	stripped := f
	stripped.ImpactScopes = nil
	if !reflect.DeepEqual(stripped, withoutScopes) {
		t.Fatalf("scope stripping changed fields beyond impact metadata")
	}
	if withoutScopes.Fingerprint != f.Fingerprint {
		t.Fatalf("fingerprint changed after adding impact scopes: %q vs %q", withoutScopes.Fingerprint, f.Fingerprint)
	}

	before := findings.NewReport("1.34", "prod", "eks", time.Unix(0, 0).UTC(), []findings.Finding{withoutScopes})
	after := findings.NewReport("1.34", "prod", "eks", time.Unix(0, 0).UTC(), []findings.Finding{f})
	beforeFinding := before.Findings[0]
	afterFinding := after.Findings[0]
	if beforeFinding.Severity != afterFinding.Severity {
		t.Fatalf("severity changed: %q -> %q", beforeFinding.Severity, afterFinding.Severity)
	}
	if beforeFinding.EffectiveUpgradeGate() != afterFinding.EffectiveUpgradeGate() {
		t.Fatalf("effective gate changed: %q -> %q", beforeFinding.EffectiveUpgradeGate(), afterFinding.EffectiveUpgradeGate())
	}
	if beforeFinding.Priority != afterFinding.Priority {
		t.Fatalf("priority changed: %q -> %q", beforeFinding.Priority, afterFinding.Priority)
	}
	if beforeFinding.CanUpgradeContinue != afterFinding.CanUpgradeContinue {
		t.Fatalf("CanUpgradeContinue changed: %v -> %v", beforeFinding.CanUpgradeContinue, afterFinding.CanUpgradeContinue)
	}
	if before.Summary != after.Summary || before.ExitCode() != after.ExitCode() || before.Result() != after.Result() {
		t.Fatalf("report gate changed: summary %+v -> %+v result %s/%d -> %s/%d",
			before.Summary, after.Summary, before.Result(), before.ExitCode(), after.Result(), after.ExitCode())
	}
	if after.Summary.Blockers != 0 || after.ExitCode() == 2 {
		t.Fatalf("impact-scope metadata created a blocker: summary %+v exit %d", after.Summary, after.ExitCode())
	}

	encoded, err := json.Marshal(after)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	for _, scope := range want {
		if !strings.Contains(string(encoded), `"`+string(scope)+`"`) {
			t.Fatalf("canonical JSON %s missing impact scope %q", encoded, scope)
		}
	}

	cmp, err := comparison.Compare(before, after)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(cmp.New) != 0 || len(cmp.Resolved) != 0 || len(cmp.Changed) != 1 {
		t.Fatalf("compare new/resolved/changed = %d/%d/%d, want 0/0/1", len(cmp.New), len(cmp.Resolved), len(cmp.Changed))
	}
	if len(cmp.Changed[0].Changes) != 1 {
		t.Fatalf("compare changes = %+v, want only impactScopes", cmp.Changed[0].Changes)
	}
	if _, ok := cmp.Changed[0].Changes["impactScopes"]; !ok {
		t.Fatalf("compare changes = %+v, want impactScopes change", cmp.Changed[0].Changes)
	}
}

func impactScopeNODE003Finding(t *testing.T) findings.Finding {
	t.Helper()
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{
		node003Deployment("payments", "legacy-pinned", corev1.PodSpec{
			NodeSelector: map[string]string{deprecatedMasterNodeLabel: ""},
		}),
	}}
	fs, err := (NODE003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("NODE003 Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("NODE003 got %d findings, want 1: %+v", len(fs), fs)
	}
	return fs[0]
}

func impactScopeDRAIN001Finding(t *testing.T) findings.Finding {
	t.Helper()
	snap := &k8s.Snapshot{Deployments: []appsv1.Deployment{{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default", UID: "uid-api"},
		Spec:       appsv1.DeploymentSpec{Replicas: int32Ptr(1)},
	}}}
	fs, err := (DRAIN001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("DRAIN001 Evaluate: %v", err)
	}
	return drain001RequireOne(t, fs)
}

func impactScopeDRAIN003Finding(t *testing.T) findings.Finding {
	t.Helper()
	snap := &k8s.Snapshot{
		Deployments: []appsv1.Deployment{drain003Deployment("gpu-app", 1, corev1.PodSpec{
			NodeSelector: map[string]string{"gpu": "true"},
		})},
		Nodes: []corev1.Node{
			drain003Node("gpu-node-1", map[string]string{"gpu": "true"}, nil),
			drain003Node("plain-node", nil, nil),
		},
	}
	fs, err := (DRAIN003{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("DRAIN003 Evaluate: %v", err)
	}
	return drain003RequireDiscriminator(t, fs, "scarcity")
}

func impactScopeDRAIN004Finding(t *testing.T) findings.Finding {
	t.Helper()
	snap := &k8s.Snapshot{
		Nodes: []corev1.Node{drain004Node("node-a", "4", "16Gi"), drain004Node("node-b", "1", "16Gi")},
		Pods:  []corev1.Pod{drain004Pod("app-1", "node-a", "3500m", "1Gi", nil)},
	}
	fs, err := (DRAIN004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("DRAIN004 Evaluate: %v", err)
	}
	return drain004RequireOne(t, fs)
}

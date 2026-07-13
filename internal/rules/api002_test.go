package rules

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestAPI002_LiveObjectBeforeRemovalCreatesWarning(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "negative")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (API002{}).Evaluate(&ScanContext{K8s: snap}, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1 deprecated warning: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.RuleID != "API-002" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierStaticCertain {
		t.Fatalf("finding identity = %+v, want API-002 warning STATIC_CERTAIN", f)
	}
	if f.Resources[0].Kind != "HorizontalPodAutoscaler" || f.Resources[0].Namespace != "payments" || f.Resources[0].Name != "payments-hpa" {
		t.Fatalf("resource = %+v, want HorizontalPodAutoscaler payments/payments-hpa", f.Resources[0])
	}
	for _, want := range []string{"apiVersion: autoscaling/v2beta2", "removed in: Kubernetes 1.26", "target version: 1.24", "status: deprecated but still served at target version"} {
		if !contains(f.Evidence, want) {
			t.Errorf("evidence missing %q: %v", want, f.Evidence)
		}
	}

	r := findings.NewReport("1.24", "cluster", "", time.Now(), fs)
	if r.Result() != "PASSED_WITH_WARNINGS" || r.ExitCode() != 1 {
		t.Fatalf("report result/exit = %s/%d, want PASSED_WITH_WARNINGS/1", r.Result(), r.ExitCode())
	}
	if r.Findings[0].Priority != string(findings.PriorityP4) || r.Findings[0].CanUpgradeContinue != true {
		t.Fatalf("priority fields = %+v, want P4 and canUpgradeContinue=true", r.Findings[0])
	}
	if r.APICompatibility == nil || r.APICompatibility.Status != "Warning" || r.APICompatibility.DeprecatedObjects != 1 || r.APICompatibility.UpgradeContinue != true {
		t.Fatalf("APICompatibility = %+v, want Warning with one deprecated object and upgradeContinue=true", r.APICompatibility)
	}
}

func TestAPI002_DoesNotFireAtOrAfterRemoval(t *testing.T) {
	dep := findDeprecatedAPI(t, "autoscaling", "v2beta2", "HorizontalPodAutoscaler")
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{{
		DeprecatedAPI: dep, Namespace: "payments", Name: "payments-hpa", UID: "hpa-uid",
	}}}}

	for _, target := range []string{"1.26", "1.27"} {
		t.Run(target, func(t *testing.T) {
			fs, err := (API002{}).Evaluate(sc, target)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(fs) != 0 {
				t.Fatalf("got %d findings, want 0 once API-001 owns removed API state: %+v", len(fs), fs)
			}
		})
	}
}

func TestAPI002_ManifestPlaneFindsDeprecatedButStillServedAPI(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "negative"))
	if err != nil {
		t.Fatalf("resolving fixture path: %v", err)
	}
	mc := manifest.NewCollector([]string{repo}, nil)
	msnap, err := mc.Collect(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("manifest Collect: %v", err)
	}

	fs, err := (API002{}).Evaluate(&ScanContext{Manifests: msnap}, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1 manifest warning: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Resources[0].Plane != findings.PlaneManifest || f.Resources[0].SourcePath != "hpa.yaml" {
		t.Fatalf("manifest resource = %+v, want source hpa.yaml", f.Resources[0])
	}
	if !strings.Contains(f.Message, "still served at target 1.24") {
		t.Fatalf("message = %q, want target served wording", f.Message)
	}
}

func TestAPI002_LiveAndManifestPlanesMerge(t *testing.T) {
	dep := findDeprecatedAPI(t, "autoscaling", "v2beta2", "HorizontalPodAutoscaler")
	sc := &ScanContext{
		K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{{
			DeprecatedAPI: dep, Namespace: "payments", Name: "payments-hpa", UID: "hpa-uid",
		}}},
		Manifests: &manifest.Snapshot{DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{{
			DeprecatedAPI: dep, Namespace: "payments", Name: "payments-hpa", SourcePath: "hpa.yaml",
		}}},
	}
	fs, err := (API002{}).Evaluate(sc, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want one merged finding: %+v", len(fs), fs)
	}
	if len(fs[0].Resources) != 2 || !hasPlane(fs[0].Resources, findings.PlaneLive) || !hasPlane(fs[0].Resources, findings.PlaneManifest) {
		t.Fatalf("resources = %+v, want live and manifest occurrences", fs[0].Resources)
	}
	if !contains(fs[0].Evidence, "cross-plane match: exact Kind+Namespace+Name identity") {
		t.Fatalf("evidence = %v, want cross-plane match note", fs[0].Evidence)
	}
}

func TestAPI002_LiveEventsSuppressed(t *testing.T) {
	dep := findDeprecatedAPI(t, "events.k8s.io", "v1beta1", "Event")
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{{
		DeprecatedAPI: dep, Namespace: "default", Name: "some-pod.abcdef123456", UID: "evt-uid",
	}}}}
	fs, err := (API002{}).Evaluate(sc, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want live Event objects suppressed: %+v", len(fs), fs)
	}
}

func TestAPI002_InvalidTargetVersion(t *testing.T) {
	_, err := (API002{}).Evaluate(&ScanContext{}, "not-a-version")
	if err == nil || !strings.Contains(err.Error(), "API-002") {
		t.Fatalf("Evaluate invalid target err = %v, want API-002 context", err)
	}
}

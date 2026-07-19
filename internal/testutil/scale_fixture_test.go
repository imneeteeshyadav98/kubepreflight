package testutil_test

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

const scaleTargetVersion = "1.34"

func TestScaleFixtureScenarioCounts(t *testing.T) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		t.Run(cfg.Name, func(t *testing.T) {
			fixture, err := testutil.GenerateScaleFixture(cfg)
			if err != nil {
				t.Fatalf("GenerateScaleFixture: %v", err)
			}
			snap := fixture.Snapshot
			assertCount(t, "namespaces", len(fixture.Namespaces), cfg.NamespaceCount)
			assertCount(t, "pods", len(snap.Pods), cfg.PodCount)
			assertCount(t, "deployments", len(snap.Deployments), cfg.DeploymentCount)
			assertCount(t, "statefulsets", len(snap.StatefulSets), cfg.StatefulSetCount)
			assertCount(t, "daemonsets", len(snap.DaemonSets), cfg.DaemonSetCount)
			assertCount(t, "pdbs", len(snap.PodDisruptionBudgets), cfg.PDBCount)
			assertCount(t, "crds", len(snap.CustomResourceDefinitions), cfg.CRDCount)
			assertCount(t, "validating webhooks", len(snap.ValidatingWebhookConfigs), cfg.AdmissionWebhookCount)
			assertCount(t, "nodes", len(snap.Nodes), cfg.NodeCount)
		})
	}
}

func TestScaleFixtureDeterministic(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("medium")
	if !ok {
		t.Fatal("medium scenario missing")
	}
	a, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateScaleFixture(a): %v", err)
	}
	b, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateScaleFixture(b): %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("repeated fixture generation is not deterministic")
	}
	if got := a.Namespaces[0].Name; got != "scale-ns-0000" {
		t.Fatalf("first namespace = %q, want scale-ns-0000", got)
	}
	if got := a.Snapshot.Pods[0].Name; got != "scale-pod-000000" {
		t.Fatalf("first pod = %q, want scale-pod-000000", got)
	}
}

func TestScaleFixtureNoDuplicateResourceIdentities(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("large")
	if !ok {
		t.Fatal("large scenario missing")
	}
	fixture, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateScaleFixture: %v", err)
	}
	seen := map[string]bool{}
	addIdentity := func(kind, namespace, name string) {
		t.Helper()
		key := kind + "/" + namespace + "/" + name
		if seen[key] {
			t.Fatalf("duplicate resource identity %s", key)
		}
		seen[key] = true
	}
	for _, ns := range fixture.Namespaces {
		addIdentity("Namespace", "", ns.Name)
	}
	for _, node := range fixture.Snapshot.Nodes {
		addIdentity("Node", "", node.Name)
	}
	for _, pod := range fixture.Snapshot.Pods {
		addIdentity("Pod", pod.Namespace, pod.Name)
	}
	for _, d := range fixture.Snapshot.Deployments {
		addIdentity("Deployment", d.Namespace, d.Name)
	}
	for _, sts := range fixture.Snapshot.StatefulSets {
		addIdentity("StatefulSet", sts.Namespace, sts.Name)
	}
	for _, ds := range fixture.Snapshot.DaemonSets {
		addIdentity("DaemonSet", ds.Namespace, ds.Name)
	}
	for _, pdb := range fixture.Snapshot.PodDisruptionBudgets {
		addIdentity("PodDisruptionBudget", pdb.Namespace, pdb.Name)
	}
	for _, crd := range fixture.Snapshot.CustomResourceDefinitions {
		addIdentity("CustomResourceDefinition", "", crd.Name)
	}
	for _, wh := range fixture.Snapshot.ValidatingWebhookConfigs {
		addIdentity("ValidatingWebhookConfiguration", "", wh.Name)
	}
}

func TestScaleFixtureLargeScanAndReports(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("large")
	if !ok {
		t.Fatal("large scenario missing")
	}
	fixture, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateScaleFixture: %v", err)
	}
	fs, err := rules.NewDefaultRegistry().RunAll(&rules.ScanContext{K8s: fixture.Snapshot}, scaleTargetVersion)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	if len(fs) == 0 {
		t.Fatal("large scale fixture produced no findings; want representative risky objects")
	}

	rpt := findings.NewReport(scaleTargetVersion, "scale-large", "", time.Unix(0, 0).UTC(), fs)
	if rpt.UpgradeReadiness == nil {
		t.Fatal("UpgradeReadiness summary is nil")
	}
	if !hasReadinessCategory(rpt, "Drain Readiness") {
		t.Fatalf("UpgradeReadiness categories = %+v, want Drain Readiness", rpt.UpgradeReadiness.Categories)
	}

	var buf bytes.Buffer
	if err := report.WriteJSON(rpt, &buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("WriteJSON produced empty output")
	}
	buf.Reset()
	if err := report.WriteMarkdown(rpt, &buf); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("WriteMarkdown produced empty output")
	}
	buf.Reset()
	if err := report.WriteHTML(rpt, &buf); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("WriteHTML produced empty output")
	}
}

func TestScaleFixtureFindingCountsStable(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("medium")
	if !ok {
		t.Fatal("medium scenario missing")
	}
	a := scaleFindings(t, cfg)
	b := scaleFindings(t, cfg)
	if len(a) != len(b) {
		t.Fatalf("finding count changed between identical fixtures: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Fingerprint != b[i].Fingerprint || a[i].RuleID != b[i].RuleID {
			t.Fatalf("finding[%d] changed: %s/%s vs %s/%s", i, a[i].RuleID, a[i].Fingerprint, b[i].RuleID, b[i].Fingerprint)
		}
	}
}

func assertCount(t *testing.T, label string, got, want int) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %d, want %d", label, got, want)
	}
}

func scaleFindings(t *testing.T, cfg testutil.ScaleFixtureConfig) []findings.Finding {
	t.Helper()
	fixture, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateScaleFixture: %v", err)
	}
	fs, err := rules.NewDefaultRegistry().RunAll(&rules.ScanContext{K8s: fixture.Snapshot}, scaleTargetVersion)
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	return fs
}

func hasReadinessCategory(rpt *findings.Report, name string) bool {
	if rpt.UpgradeReadiness == nil {
		return false
	}
	for _, cat := range rpt.UpgradeReadiness.Categories {
		if cat.Name == name {
			return true
		}
	}
	return false
}

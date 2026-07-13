// Package integration exercises the same collect -> rules -> report chain
// internal/cli/scan.go wires against a real kubeconfig, but against fixture
// data instead of a live cluster. This establishes the "fixtures -> pipeline
// -> assertions" pattern Week 2's checks extend with real findings.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"
	"time"

	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/report"
	"kubepreflight/internal/rules"
	"kubepreflight/internal/testutil"
)

func TestScanPipeline_FixturesToJSON(t *testing.T) {
	fixturesDir := filepath.Join("..", "..", "testdata", "fixtures")
	objs, err := testutil.LoadFixtures(fixturesDir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	coreObjs, crdObjs := testutil.SplitCRDs(objs)

	client := fake.NewSimpleClientset(coreObjs...)
	apiExtCli := apiextensionsfake.NewSimpleClientset(crdObjs...)
	dynamicClient := testutil.NewFakeDynamicClient()

	collector := k8s.NewCollector(client, apiExtCli, dynamicClient)
	snap, err := collector.Collect(context.Background(), k8s.DefaultCollectorTimeout, k8s.DefaultCollectorConcurrency)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}

	registry := rules.NewDefaultRegistry()
	sc := &rules.ScanContext{K8s: snap} // AWS nil: not attempted in this scan, must not error
	fs, err := registry.RunAll(sc, "1.34")
	if err != nil {
		t.Fatalf("RunAll: %v", err)
	}
	// The Week 1 fixture set encodes three scenarios at once:
	//  - payments-guard: catch-all scope (apiGroups/resources/operations
	//    all "*") + fail-closed + dead backend co-fires WH-001 (Warning)
	//    and WH-002 (Blocker) on the same webhook (expected — different
	//    evidence, not deduped). The fixture's clientConfig also has no
	//    caBundle set, which is itself a legitimate (if noteworthy)
	//    configuration -- WH-004 (Warning) fires alongside WH-001/WH-002 on
	//    the same webhook for the same reason: distinct evidence about the
	//    same object, not deduped. The same catch-all scope also fires
	//    WH-005 three times: operations: ["*"] (Warning: wildcard
	//    operations, includes CONNECT), the wildcard also covers
	//    admissionregistration.k8s.io/validatingwebhookconfigurations
	//    itself (Blocker: self-interception, GlobalBlocker), and covers
	//    core/nodes,namespaces,persistentvolumes (Blocker: fail-closed
	//    high-risk-resource-scope, CriticalInfra) — three more distinct,
	//    independently-actionable facts about the same broad scope.
	//  - payments-pdb: disruptionsAllowed=0 fires PDB-001 (Blocker)
	// PDB-002 and NODE-001 don't fire: there's only one PDB in the fixture
	// set (nothing to overlap with), and the fixture node's kubelet skew is
	// within the n-3 policy against target 1.34.
	wantRuleIDs := []string{"PDB-001", "WH-001", "WH-002", "WH-004", "WH-005", "WH-005", "WH-005"}
	gotRuleIDs := make([]string, len(fs))
	for i, f := range fs {
		gotRuleIDs[i] = f.RuleID
	}
	sort.Strings(gotRuleIDs)
	if len(gotRuleIDs) != len(wantRuleIDs) {
		t.Fatalf("got %d findings %v, want %d %v", len(gotRuleIDs), gotRuleIDs, len(wantRuleIDs), wantRuleIDs)
	}
	for i := range wantRuleIDs {
		if gotRuleIDs[i] != wantRuleIDs[i] {
			t.Errorf("Findings RuleIDs = %v, want %v", gotRuleIDs, wantRuleIDs)
			break
		}
	}

	rpt := findings.NewReport("1.34", "test-context", "", time.Now().UTC(), fs)

	var buf bytes.Buffer
	if err := report.WriteJSON(rpt, &buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var decoded findings.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding written JSON: %v", err)
	}
	if decoded.TargetVersion != "1.34" {
		t.Errorf("TargetVersion = %q, want 1.34", decoded.TargetVersion)
	}
	// WH-002 and PDB-001 are Blockers; WH-001 and WH-004 (no caBundle set)
	// are Warnings. WH-005 adds two more Blockers (self-interception,
	// high-risk-resource-scope) and one more Warning (wildcard operations).
	if decoded.Summary.Blockers != 4 || decoded.Summary.Warnings != 3 {
		t.Errorf("Summary = %+v, want {Blockers:4 Warnings:3}", decoded.Summary)
	}
	if len(decoded.Findings) != 7 {
		t.Errorf("Findings = %d, want 7", len(decoded.Findings))
	}
}

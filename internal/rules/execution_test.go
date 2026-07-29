package rules

import (
	"errors"
	"sort"
	"strings"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/manifest"
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
	sc := &ScanContext{
		K8s:                 &k8s.Snapshot{},
		AWS:                 &aws.Snapshot{},
		Manifests:           &manifest.Snapshot{},
		KubernetesRequested: true,
		AWSRequested:        true,
		ManifestsRequested:  true,
	}

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
	sc := &ScanContext{Manifests: &manifest.Snapshot{}, ManifestsRequested: true}

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
		"default":        {registry: NewDefaultRegistry(), sc: &ScanContext{K8s: &k8s.Snapshot{}, AWS: &aws.Snapshot{}, Manifests: &manifest.Snapshot{}, KubernetesRequested: true, AWSRequested: true, ManifestsRequested: true}},
		"manifests-only": {registry: NewManifestsOnlyRegistry(), sc: &ScanContext{Manifests: &manifest.Snapshot{}, ManifestsRequested: true}},
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

func TestNewDefaultRegistry_EveryRuleDeclaresDependencyContract(t *testing.T) {
	registry := NewDefaultRegistry()
	seen := make(map[string]int, len(registry.rules))
	for _, rule := range registry.rules {
		seen[rule.ID()]++
		if _, ok := rule.(ContextDependencyRule); ok {
			continue
		}
		if _, ok := rule.(DependencyRule); !ok {
			t.Fatalf("%s does not declare an explicit evidence dependency contract", rule.ID())
		}
	}

	for _, ruleID := range AllRuleIDs() {
		if seen[ruleID] != 1 {
			t.Fatalf("%s appears %d times in NewDefaultRegistry, want exactly once", ruleID, seen[ruleID])
		}
	}
	if len(seen) != len(AllRuleIDs()) {
		t.Fatalf("NewDefaultRegistry has %d unique rule IDs, AllRuleIDs has %d", len(seen), len(AllRuleIDs()))
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

type fakeFindingRule struct {
	id string
}

func (f fakeFindingRule) ID() string { return f.id }

func (f fakeFindingRule) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	ref := findings.LiveResource("ConfigMap", findings.ScopeNamespaced, "default", "demo", "uid-demo")
	return []findings.Finding{{
		RuleID:      f.id,
		Severity:    findings.SeverityInfo,
		Confidence:  findings.TierStaticCertain,
		Message:     "synthetic finding",
		Resources:   []findings.ResourceReference{ref},
		Evidence:    []string{"synthetic evidence"},
		Remediation: "synthetic remediation",
		Fingerprint: findings.FingerprintV2(f.id, targetVersion, "", ref),
	}}, nil
}

func TestRunAllWithExecutions_AllRuleErrorsMarkedFailed(t *testing.T) {
	err1 := errors.New("rule A failed")
	err2 := errors.New("rule B failed")

	registry := NewRegistry()
	registry.Register(fakeErrRule{id: "API-001", err: err1})
	registry.Register(fakeErrRule{id: "API-002", err: err2})

	_, records, err := registry.RunAllWithExecutions(&ScanContext{}, "1.34")
	if !errors.Is(err, err1) {
		t.Fatalf("RunAllWithExecutions() error = %v, want it to wrap the first rule's error", err)
	}
	if !errors.Is(err, err2) {
		t.Fatalf("RunAllWithExecutions() error = %v, want it to wrap the second rule's error too", err)
	}

	byID := recordsByRuleID(records)

	first, ok := byID["API-001"]
	if !ok {
		t.Fatal("API-001 missing from RuleExecutions")
	}
	if first.Applicability != findings.ApplicabilityApplicable || first.State != findings.ExecutionFailed {
		t.Errorf("API-001 = %+v, want Applicable/Failed", first)
	}
	if first.Reason == "" {
		t.Error("API-001 Reason is empty, want the sanitized error text")
	}

	second, ok := byID["API-002"]
	if !ok {
		t.Fatal("API-002 missing from RuleExecutions")
	}
	if second.Applicability != findings.ApplicabilityApplicable || second.State != findings.ExecutionFailed {
		t.Errorf("API-002 = %+v, want Applicable/Failed", second)
	}
	if second.Reason == "" {
		t.Error("API-002 Reason is empty, want the sanitized error text")
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

func TestRunAllWithExecutions_FailedRuleReasonIsSanitized(t *testing.T) {
	rawErr := errors.New("kubeconfig /home/alice/.kube/config hit https://10.1.2.3:6443 using Bearer abc.def and arn:aws:iam::123456789012:role/Admin via ip-10-1-2-3.ec2.internal on C:\\Users\\alice\\.aws\\credentials")
	registry := NewRegistry()
	registry.Register(fakeErrRule{id: "API-001", err: rawErr})

	_, records, err := registry.RunAllWithExecutions(&ScanContext{}, "1.34")
	if !errors.Is(err, rawErr) {
		t.Fatalf("RunAllWithExecutions() error = %v, want raw error preserved for programmatic callers", err)
	}

	rec := recordsByRuleID(records)["API-001"]
	if rec.State != findings.ExecutionFailed {
		t.Fatalf("API-001 = %+v, want failed", rec)
	}
	for _, leaked := range []string{
		"/home/alice/.kube/config",
		"https://10.1.2.3:6443",
		"Bearer abc.def",
		"arn:aws:iam::123456789012:role/Admin",
		"123456789012",
		"ip-10-1-2-3.ec2.internal",
		"C:\\Users\\alice\\.aws\\credentials",
	} {
		if strings.Contains(rec.Reason, leaked) {
			t.Fatalf("sanitized reason %q leaked %q", rec.Reason, leaked)
		}
	}
	for _, marker := range []string{
		"[REDACTED_PATH]",
		"[REDACTED_URL]",
		"Bearer [REDACTED]",
		"[REDACTED_ARN]",
		"[REDACTED_PRIVATE_HOSTNAME]",
	} {
		if !strings.Contains(rec.Reason, marker) {
			t.Fatalf("sanitized reason %q missing marker %q", rec.Reason, marker)
		}
	}
}

func TestRunAllWithExecutions_ContinuesAndClassifiesMixedRuleOutcomes(t *testing.T) {
	ruleErr := errors.New("synthetic failure")
	registry := NewRegistry()
	registry.Register(fakeErrRule{id: "API-001"})
	registry.Register(fakeFindingRule{id: "API-002"})
	registry.Register(fakeErrRule{id: "WH-001", err: ruleErr})
	registry.Register(fakeErrRule{id: "WH-002"})
	registry.Register(PDB001{})

	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{"poddisruptionbudgets": errors.New("forbidden")},
	}}
	fs, records, err := registry.RunAllWithExecutions(sc, "1.34")
	if !errors.Is(err, ruleErr) {
		t.Fatalf("RunAllWithExecutions() error = %v, want synthetic failure", err)
	}
	if len(fs) != 1 || fs[0].RuleID != "API-002" {
		t.Fatalf("findings = %+v, want only API-002 synthetic finding", fs)
	}

	byID := recordsByRuleID(records)
	for _, ruleID := range []string{"API-001", "API-002", "WH-002"} {
		if rec := byID[ruleID]; rec.Applicability != findings.ApplicabilityApplicable || rec.State != findings.ExecutionEvaluated {
			t.Fatalf("%s = %+v, want applicable/evaluated", ruleID, rec)
		}
	}
	if rec := byID["WH-001"]; rec.State != findings.ExecutionFailed {
		t.Fatalf("WH-001 = %+v, want failed", rec)
	}
	if rec := byID["PDB-001"]; rec.State != findings.ExecutionInsufficientEvidence {
		t.Fatalf("PDB-001 = %+v, want insufficient_evidence", rec)
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

func TestRunAllWithExecutions_ADDON002_MarkedInsufficientEvidence(t *testing.T) {
	registry := NewRegistry()
	registry.Register(ADDON002{})

	sc := &ScanContext{AWS: &aws.Snapshot{
		Errors: map[string]error{"describe-addon-versions:coredns": errors.New("access denied")},
	}}

	_, records, err := registry.RunAllWithExecutions(sc, "1.34")
	if err != nil {
		t.Fatalf("RunAllWithExecutions() error = %v, want nil", err)
	}
	rec, ok := recordsByRuleID(records)["ADDON-002"]
	if !ok {
		t.Fatal("ADDON-002 missing from RuleExecutions")
	}
	if rec.State != findings.ExecutionInsufficientEvidence {
		t.Fatalf("ADDON-002 State = %q, want insufficient_evidence", rec.State)
	}
	if !strings.Contains(rec.Reason, "describe-addon-versions") {
		t.Fatalf("ADDON-002 Reason = %q, want AWS dependency key", rec.Reason)
	}
}

func TestRunAllWithExecutions_ConditionalDependenciesDoNotOverreadEmptyInventories(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		snap *k8s.Snapshot
	}{
		{
			name: "WH-002 no webhook configs ignores service endpoint failures",
			rule: WH002{},
			snap: &k8s.Snapshot{Errors: map[string]error{
				"services":       errors.New("forbidden"),
				"endpointslices": errors.New("forbidden"),
			}},
		},
		{
			name: "CRD-002 no CRDs ignores service endpoint failures",
			rule: CRD002{},
			snap: &k8s.Snapshot{Errors: map[string]error{
				"services":       errors.New("forbidden"),
				"endpointslices": errors.New("forbidden"),
			}},
		},
		{
			name: "DRAIN-001 no singleton workloads ignores pod and pdb failures",
			rule: DRAIN001{},
			snap: &k8s.Snapshot{Errors: map[string]error{
				"pods":                 errors.New("forbidden"),
				"poddisruptionbudgets": errors.New("forbidden"),
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry()
			registry.Register(tc.rule)
			_, records, err := registry.RunAllWithExecutions(&ScanContext{K8s: tc.snap}, "1.34")
			if err != nil {
				t.Fatalf("RunAllWithExecutions() error = %v, want nil", err)
			}
			rec := recordsByRuleID(records)[tc.rule.ID()]
			if rec.Applicability != findings.ApplicabilityApplicable || rec.State != findings.ExecutionEvaluated {
				t.Fatalf("%s = %+v, want applicable/evaluated", tc.rule.ID(), rec)
			}
		})
	}
}

func TestRunAllWithExecutions_ConditionalDependenciesStillCatchRelevantMissingEvidence(t *testing.T) {
	serviceRef := admissionregistrationv1.ServiceReference{Namespace: "guard", Name: "guard-svc"}
	whSnap := &k8s.Snapshot{
		ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{{
			ObjectMeta: metav1.ObjectMeta{Name: "guard", UID: "uid-guard"},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{{
				Name:         "guard.example.com",
				ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &serviceRef},
			}},
		}},
		Errors: map[string]error{"services": errors.New("forbidden")},
	}

	crdSnap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{{
			ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-widgets"},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Conversion: &apiextensionsv1.CustomResourceConversion{
					Strategy: apiextensionsv1.WebhookConverter,
					Webhook: &apiextensionsv1.WebhookConversion{
						ClientConfig: &apiextensionsv1.WebhookClientConfig{
							Service: &apiextensionsv1.ServiceReference{Namespace: "crd-system", Name: "converter"},
						},
						ConversionReviewVersions: []string{"v1"},
					},
				},
			},
		}},
		Errors: map[string]error{"services": errors.New("forbidden")},
	}

	for _, tc := range []struct {
		rule Rule
		snap *k8s.Snapshot
	}{
		{WH002{}, whSnap},
		{CRD002{}, crdSnap},
	} {
		registry := NewRegistry()
		registry.Register(tc.rule)
		_, records, err := registry.RunAllWithExecutions(&ScanContext{K8s: tc.snap}, "1.34")
		if err != nil {
			t.Fatalf("%s RunAllWithExecutions() error = %v, want nil", tc.rule.ID(), err)
		}
		rec := recordsByRuleID(records)[tc.rule.ID()]
		if rec.State != findings.ExecutionInsufficientEvidence || !strings.Contains(rec.Reason, "services") {
			t.Fatalf("%s = %+v, want insufficient_evidence mentioning services", tc.rule.ID(), rec)
		}
	}
}

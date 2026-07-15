package rules

import (
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/apicatalog"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

// --- PR 2/4: API-001/API-002 catalog-backed decision integration ---
//
// These tests exercise internal/rules/api_catalog.go's resolveAPIRemoval
// against real catalog-covered kinds (policy/v1beta1 PodSecurityPolicy,
// flowcontrol.apiserver.k8s.io/v1beta3 FlowSchema). Since PR 4 migrated
// the complete legacy inventory into the versioned catalog and made
// apicatalog.Deprecated a derived view of it (see apicatalog.legacyFromVersioned),
// every object either rule ever sees now resolves via an actual catalog
// hit — there is no more static-fallback path to exercise separately;
// every finding carries catalog source/reference evidence unconditionally.

func TestAPICatalogIntegration_RemovedAtTarget_API001Blocker(t *testing.T) {
	psp := findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy")
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
		{DeprecatedAPI: psp, Name: "restricted", UID: "psp-uid"},
	}}}

	// target 1.34 is inside the versioned catalog's supported range
	// (1.25-1.39) for this kind, so this must resolve via the catalog hit,
	// not the static apicatalog.Deprecated fallback.
	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("got %+v, want exactly one Blocker", fs)
	}
	if !contains(fs[0].Evidence, "removed in: Kubernetes 1.25") {
		t.Errorf("evidence = %v, want removed-in-1.25", fs[0].Evidence)
	}
	if !containsPrefixMatch(fs[0].Evidence, "catalog source: ") || !containsPrefixMatch(fs[0].Evidence, "catalog reference: ") {
		t.Errorf("evidence = %v, want catalog source/reference lines for a catalog-verified hit", fs[0].Evidence)
	}

	// Report/action-plan output unchanged: catalog evidence lines are
	// appended, not substituted, so the "apiVersion: " prefix parse
	// BuildAPICompatibilitySummary relies on must still work, and priority/
	// upgrade-continue must be exactly as before.
	r := findings.NewReport("1.34", "cluster", "", time.Now(), fs)
	if r.APICompatibility == nil || r.APICompatibility.RemovedObjects != 1 || r.APICompatibility.Status != "Failed" {
		t.Fatalf("APICompatibility = %+v, want Failed with one removed object", r.APICompatibility)
	}
	if len(r.APICompatibility.RemovedFamilies) != 1 || r.APICompatibility.RemovedFamilies[0].APIVersion != "policy/v1beta1" {
		t.Fatalf("RemovedFamilies = %+v, want policy/v1beta1 parsed from evidence", r.APICompatibility.RemovedFamilies)
	}
	if r.Findings[0].Priority != string(findings.PriorityP2) || r.Findings[0].CanUpgradeContinue != false {
		t.Fatalf("priority fields = %+v, want P2 and canUpgradeContinue=false", r.Findings[0])
	}
	if r.Result() != "BLOCKED" || r.ExitCode() != 2 {
		t.Fatalf("report result/exit = %s/%d, want BLOCKED/2", r.Result(), r.ExitCode())
	}
	// Action-plan placement for API-001 findings is unchanged: this
	// finding's shape (RuleID, Severity, Fingerprint) is identical to the
	// pre-catalog code path — only Evidence gained extra lines — and
	// internal/plan's own test suite (unmodified by this change, still
	// green) already locks down fix-api-compatibility's projection from
	// API-001/API-002 by RuleID.
}

func TestAPICatalogIntegration_DeprecatedStillServed_API002Warning(t *testing.T) {
	psp := findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy")
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
		{DeprecatedAPI: psp, Name: "restricted", UID: "psp-uid"},
	}}}

	// target 1.24 is below this entry's own SupportedTargetRange.Min
	// (1.25), but EntryFor (unlike Lookup) isn't range-gated — the entry
	// is still found, and its RemovedInVersion (1.25) correctly resolves
	// to "not yet removed at 1.24" via targetBeforeRemoval.
	fs, err := (API002{}).Evaluate(sc, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityWarning {
		t.Fatalf("got %+v, want exactly one Warning", fs)
	}
	if !containsPrefixMatch(fs[0].Evidence, "catalog source: ") {
		t.Errorf("evidence = %v, want catalog source line — every decision is catalog-backed now", fs[0].Evidence)
	}

	// The complementary API-001 run at the same target must stay silent —
	// mutual exclusion holds across the catalog-hit/fallback boundary too.
	f1, err := (API001{}).Evaluate(sc, "1.24")
	if err != nil {
		t.Fatalf("API-001 Evaluate: %v", err)
	}
	if len(f1) != 0 {
		t.Fatalf("API-001 fired %+v at target before removal, want none", f1)
	}
}

func TestAPICatalogIntegration_SupportedAPI_NeitherRuleFires(t *testing.T) {
	// A live object at a current (non-deprecated) API version never enters
	// DeprecatedAPIUsage in the first place — this is the collector-level
	// guarantee both rules depend on for "supported API -> no finding".
	// TestAPI001_Negative_UnmatchedObjectNotCollected already proves this
	// for API-001's own fixtures; this closes the same gap explicitly for
	// API-002 too.
	dir := "../../testdata/fixtures/checks/wh001/positive"
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)
	if len(snap.DeprecatedAPIUsage) != 0 {
		t.Fatalf("got %d DeprecatedAPIUsage entries from unrelated fixtures, want 0", len(snap.DeprecatedAPIUsage))
	}

	sc := &ScanContext{K8s: snap}
	f1, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("API-001 Evaluate: %v", err)
	}
	f2, err := (API002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("API-002 Evaluate: %v", err)
	}
	if len(f1) != 0 || len(f2) != 0 {
		t.Fatalf("supported-API object fired findings: API-001=%+v API-002=%+v, want none", f1, f2)
	}
}

func TestAPICatalogIntegration_MutualExclusion_AcrossTargets(t *testing.T) {
	psp := findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy") // removed 1.25
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
		{DeprecatedAPI: psp, Name: "restricted", UID: "psp-uid"},
	}}}

	for _, target := range []string{"1.20", "1.24", "1.25", "1.26", "1.34", "1.39", "1.45"} {
		t.Run(target, func(t *testing.T) {
			f1, err := (API001{}).Evaluate(sc, target)
			if err != nil {
				t.Fatalf("API-001 Evaluate: %v", err)
			}
			f2, err := (API002{}).Evaluate(sc, target)
			if err != nil {
				t.Fatalf("API-002 Evaluate: %v", err)
			}
			if len(f1) > 0 && len(f2) > 0 {
				t.Fatalf("target %s: both API-001 (%d) and API-002 (%d) fired for the same object", target, len(f1), len(f2))
			}
			if len(f1)+len(f2) != 1 {
				t.Fatalf("target %s: exactly one of API-001/API-002 must fire for a known-deprecated object, got API-001=%d API-002=%d", target, len(f1), len(f2))
			}
		})
	}
}

func TestAPICatalogIntegration_LiveAndManifestResultsMatch(t *testing.T) {
	psp := findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy")

	// target 1.45 is beyond this entry's own SupportedTargetRange.Max
	// (1.39), but EntryFor isn't range-gated — both planes independently
	// resolve the same catalog entry and must still agree with each
	// other and merge into one finding.
	sc := &ScanContext{
		K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: psp, Name: "shared-name", UID: "live-uid"},
		}},
		Manifests: &manifest.Snapshot{DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{
			{DeprecatedAPI: psp, Name: "shared-name", SourcePath: "psp.yaml"},
		}},
	}
	fs, err := (API001{}).Evaluate(sc, "1.45")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want one merged conceptual finding: %+v", len(fs), fs)
	}
	if !hasPlane(fs[0].Resources, findings.PlaneLive) || !hasPlane(fs[0].Resources, findings.PlaneManifest) {
		t.Fatalf("resources = %+v, want both live and manifest occurrences merged", fs[0].Resources)
	}
	if !contains(fs[0].Evidence, "removed in: Kubernetes 1.25") {
		t.Errorf("evidence = %v, want both planes to agree on removed-in-1.25 (out-of-range fallback)", fs[0].Evidence)
	}
}

func TestAPICatalogIntegration_TargetOutsideCatalogRange_NotFalseClean(t *testing.T) {
	// policy/v1beta1 PodSecurityPolicy IS catalog-covered, but its own
	// SupportedTargetRange tops out at 1.39. EntryFor isn't range-gated,
	// so this must still resolve to a real Blocker (never silently CLEAN)
	// for a target beyond that entry's verified range — PR 3's CLI-level
	// buildSupportedTargetRange gate is the thing that actually stops a
	// scan at target 1.45 from reaching this rule at all in practice, but
	// this rule-level guard must hold independently too.
	psp := findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy")
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
		{DeprecatedAPI: psp, Name: "restricted", UID: "psp-uid"},
	}}}

	fs, err := (API001{}).Evaluate(sc, "1.45")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("got %+v, want exactly one Blocker for a target beyond the catalog's verified range", fs)
	}
	if !containsPrefixMatch(fs[0].Evidence, "catalog source: ") {
		t.Errorf("evidence = %v, want catalog source line — every decision is catalog-backed now", fs[0].Evidence)
	}
}

// TestAPICatalogIntegration_AutoManagedFlowControl_CatalogVerified proves
// the APF auto-managed noise-suppression special case (Info, not Blocker)
// survives an actual catalog hit — every existing regression test for this
// behavior (api001_test.go) uses the uncatalogued v1beta1 FlowSchema, which
// only exercises the fallback path.
func TestAPICatalogIntegration_AutoManagedFlowControl_CatalogVerified(t *testing.T) {
	dep := findDeprecatedAPI(t, "flowcontrol.apiserver.k8s.io", "v1beta3", "FlowSchema") // catalog range 1.32-1.39
	sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
		{DeprecatedAPI: dep, Name: "exempt", UID: "fs-uid", AutoManaged: true},
	}}}

	fs, err := (API001{}).Evaluate(sc, "1.32")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Severity != findings.SeverityInfo {
		t.Errorf("Severity = %q, want Info for a kube-apiserver-managed default even through a catalog hit", f.Severity)
	}
	if !containsPrefixMatch(f.Evidence, "catalog source: ") {
		t.Errorf("evidence = %v, want catalog source line for this catalog-covered target", f.Evidence)
	}
	if f.RemediationDetail != nil {
		t.Errorf("RemediationDetail = %+v, want nil", f.RemediationDetail)
	}
}

// TestAPICatalogIntegration_MissingCatalogEntryIsIntegrityError proves the
// PR 4 completeness guard: apicatalog.Deprecated is derived from the
// versioned catalog, so a live/manifest object at a group/version/kind
// the catalog doesn't know about can only mean the two have drifted out
// of sync — a bug, never a silent "treat as compatible" or a quiet
// fallback. This can't happen through the real collector-fed path
// anymore (that's the whole point), so it's simulated directly by
// constructing a DeprecatedAPI the catalog was never given, the same way
// a future accidental edit to legacyDeprecatedSnapshot without a matching
// versioned_catalog.json entry would surface.
func TestAPICatalogIntegration_MissingCatalogEntryIsIntegrityError(t *testing.T) {
	phantom := apicatalog.DeprecatedAPI{
		Group: "phantom.example.com", Version: "v1beta1", Resource: "phantoms", Kind: "Phantom",
		RemovedInVersion: "1.25", Replacement: "phantom.example.com/v1 Phantom", ReplacementAPIVersion: "phantom.example.com/v1",
	}

	t.Run("API-001 live", func(t *testing.T) {
		sc := &ScanContext{K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: phantom, Name: "ghost", UID: "phantom-uid"},
		}}}
		_, err := (API001{}).Evaluate(sc, "1.34")
		if err == nil || !strings.Contains(err.Error(), "integrity error") {
			t.Fatalf("Evaluate error = %v, want a catalog integrity error", err)
		}
	})

	t.Run("API-002 manifest", func(t *testing.T) {
		sc := &ScanContext{Manifests: &manifest.Snapshot{DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{
			{DeprecatedAPI: phantom, Name: "ghost", SourcePath: "ghost.yaml"},
		}}}
		_, err := (API002{}).Evaluate(sc, "1.20")
		if err == nil || !strings.Contains(err.Error(), "integrity error") {
			t.Fatalf("Evaluate error = %v, want a catalog integrity error", err)
		}
	})
}

func containsPrefixMatch(values []string, prefix string) bool {
	for _, v := range values {
		if len(v) >= len(prefix) && v[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

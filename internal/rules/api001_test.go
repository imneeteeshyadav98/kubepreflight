package rules

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/apicatalog"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestAPI001_Positive_LiveObjectAtRemovedAPI(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "positive")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("BuildSnapshot matched %d deprecated-API objects, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	// policy/v1beta1 PodSecurityPolicy was removed in Kubernetes 1.25;
	// target 1.34 has long since passed that, so this must fire.
	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "API-001" {
		t.Errorf("RuleID = %q, want API-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", f.Confidence)
	}
	if f.Resources[0].Kind != "PodSecurityPolicy" || f.Resources[0].Name != "restricted" {
		t.Errorf("Resources = %+v, want PodSecurityPolicy/restricted", f.Resources)
	}
}

func TestAPI001_Negative_TargetVersionBeforeRemoval(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "negative")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("BuildSnapshot matched %d deprecated-API objects, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	// autoscaling/v2beta2 HPA is removed in Kubernetes 1.26. Target 1.24
	// hasn't reached that yet, so this object is still perfectly valid at
	// the target version and must not fire.
	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.24")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (target version before removal must not fire): %+v", len(fs), fs)
	}
}

func TestAPI001_Negative_UnmatchedObjectNotCollected(t *testing.T) {
	// A live object at a *current* API version should never even reach
	// DeprecatedAPIUsage — BuildSnapshot only records objects whose GVK
	// matches an apicatalog.Deprecated entry.
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "wh001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)
	if len(snap.DeprecatedAPIUsage) != 0 {
		t.Fatalf("got %d DeprecatedAPIUsage entries from unrelated fixtures, want 0: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}
}

func TestAPI001_Positive_ManifestPlaneFindsDeprecatedAPI(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "manifest-repo"))
	if err != nil {
		t.Fatalf("resolving fixture repo path: %v", err)
	}
	mc := manifest.NewCollector([]string{filepath.Join(repo, "raw")}, nil)
	msnap, err := mc.Collect(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("manifest Collect: %v", err)
	}

	sc := &ScanContext{K8s: &k8s.Snapshot{Errors: map[string]error{}}, Manifests: msnap}
	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "API-001" {
		t.Errorf("RuleID = %q, want API-001", f.RuleID)
	}
	if f.Resources[0].Kind != "PodSecurityPolicy" || f.Resources[0].Name != "manifest-restricted" {
		t.Errorf("Resources = %+v, want PodSecurityPolicy/manifest-restricted", f.Resources)
	}
	if f.Resources[0].UID != "" || f.Resources[0].Plane != findings.PlaneManifest {
		t.Errorf("manifest resource = %+v, want manifest plane with no UID", f.Resources[0])
	}
	found := false
	for _, e := range f.Evidence {
		if strings.Contains(e, "source:") && strings.Contains(e, "psp.yaml") {
			found = true
		}
	}
	if !found {
		t.Errorf("evidence must cite the source file path, got: %v", f.Evidence)
	}

	// End-to-end guard that the collector's relativized SourcePath (repo
	// was resolved to an absolute path above) is what actually flows
	// through into every rendered field — Resources, Message, and
	// Evidence must all be consistent and none may leak the fixture's
	// absolute directory.
	if f.Resources[0].SourcePath != "psp.yaml" {
		t.Errorf("Resources[0].SourcePath = %q, want exactly %q", f.Resources[0].SourcePath, "psp.yaml")
	}
	if strings.Contains(f.Message, repo) {
		t.Errorf("Message leaks the absolute scan root %q: %q", repo, f.Message)
	}
	for _, e := range f.Evidence {
		if strings.Contains(e, repo) {
			t.Errorf("Evidence entry leaks the absolute scan root %q: %q", repo, e)
		}
	}
}

func TestAPI001_NilManifestsPlaneNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{K8s: &k8s.Snapshot{Errors: map[string]error{}}, Manifests: nil}
	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when Manifests plane wasn't attempted: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0: %+v", len(fs), fs)
	}
}

func TestAPI001_LiveAndManifestPlanes_MergeWithBothOccurrences(t *testing.T) {
	liveSnap := &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{
				DeprecatedAPI: apicatalog.Deprecated[0], // policy/v1beta1 PodSecurityPolicy
				Name:          "manifest-restricted",
				UID:           "live-uid-1",
			},
		},
	}
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "manifest-repo"))
	if err != nil {
		t.Fatalf("resolving fixture repo path: %v", err)
	}
	mc := manifest.NewCollector([]string{filepath.Join(repo, "raw")}, nil)
	msnap, err := mc.Collect(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("manifest Collect: %v", err)
	}

	sc := &ScanContext{K8s: liveSnap, Manifests: msnap}
	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1 merged conceptual finding: %+v", len(fs), fs)
	}
	if len(fs[0].Resources) != 2 || !hasPlane(fs[0].Resources, findings.PlaneLive) || !hasPlane(fs[0].Resources, findings.PlaneManifest) {
		t.Errorf("merged finding resources = %+v, want live and manifest occurrences", fs[0].Resources)
	}
}

func TestAPI001_DifferentOrOmittedNamespaceDoesNotMerge(t *testing.T) {
	dep := apicatalog.Deprecated[1] // namespaced extensions/v1beta1 Deployment
	for _, tc := range []struct {
		name              string
		manifestNamespace string
	}{
		{name: "different namespace", manifestNamespace: "staging"},
		{name: "omitted namespace", manifestNamespace: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := &ScanContext{
				K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{{
					DeprecatedAPI: dep, Namespace: "payments", Name: "legacy-app", UID: "live-uid",
				}}},
				Manifests: &manifest.Snapshot{DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{{
					DeprecatedAPI: dep, Namespace: tc.manifestNamespace, Name: "legacy-app", SourcePath: "deployment.yaml",
				}}},
			}
			fs, err := (API001{}).Evaluate(sc, "1.34")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(fs) != 2 {
				t.Fatalf("got %d findings, want 2 unmerged occurrences: %+v", len(fs), fs)
			}
			if fs[0].Fingerprint == fs[1].Fingerprint {
				t.Errorf("different/omitted namespaces unexpectedly share fingerprint %q", fs[0].Fingerprint)
			}
		})
	}
}

func TestAPI001_ExactNamespacedIdentityMerges(t *testing.T) {
	dep := apicatalog.Deprecated[1]
	sc := &ScanContext{
		K8s: &k8s.Snapshot{DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{{
			DeprecatedAPI: dep, Namespace: "payments", Name: "legacy-app", UID: "live-uid",
		}}},
		Manifests: &manifest.Snapshot{DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{{
			DeprecatedAPI: dep, Namespace: "payments", Name: "legacy-app", SourcePath: "deployment.yaml",
		}}},
	}
	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || len(fs[0].Resources) != 2 {
		t.Fatalf("findings = %+v, want one exact-namespace match with two occurrences", fs)
	}
}

// TestAPI001_Positive_PolicyV1beta1PodDisruptionBudget is a regression test
// for a real gap found during live EKS testing: policy/v1beta1
// PodDisruptionBudget was removed in Kubernetes 1.25 (the same wave as
// PodSecurityPolicy) but had no apicatalog entry, so it silently didn't
// fire. The fixture is the actual manifest content from that test run.
func TestAPI001_Positive_PolicyV1beta1PodDisruptionBudget(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "positive-pdb")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("BuildSnapshot matched %d deprecated-API objects, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].Resources[0].Kind != "PodDisruptionBudget" || fs[0].Resources[0].Name != "old-pdb-api" {
		t.Errorf("Resources[0] = %+v, want PodDisruptionBudget/old-pdb-api", fs[0].Resources[0])
	}
}

func TestAPI001_RemediationDetail_PolicyV1beta1PDB(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "positive-pdb")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	rd := fs[0].RemediationDetail
	if rd == nil {
		t.Fatalf("RemediationDetail = nil, want populated (policy/v1beta1 -> policy/v1 is a direct apiVersion swap)")
	}
	if len(rd.Changes) != 1 || rd.Changes[0].Field != "apiVersion" || rd.Changes[0].Current != "policy/v1beta1" || rd.Changes[0].Required != "policy/v1" {
		t.Errorf("Changes = %+v, want [{apiVersion policy/v1beta1 policy/v1}]", rd.Changes)
	}
	wantDiff := "- apiVersion: policy/v1beta1\n+ apiVersion: policy/v1"
	if rd.Diff != wantDiff {
		t.Errorf("Diff = %q, want %q", rd.Diff, wantDiff)
	}
	if rd.SafeFix == nil || len(rd.SafeFix.Steps) == 0 || rd.SafeFix.Command != "" {
		t.Errorf("SafeFix = %+v, want migration steps without a misleading placeholder command", rd.SafeFix)
	}
	if rd.VerifyCommand == "" {
		t.Error("VerifyCommand is empty, want a rerun command")
	}
}

// TestAPI001_RemediationDetail_NilWhenNoDirectVersionSwap guards that
// RemediationDetail stays nil for PodSecurityPolicy: its replacement is a
// different admission mechanism, not a version bump, so no diff can be
// honestly shown.
func TestAPI001_RemediationDetail_NilWhenNoDirectVersionSwap(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "api001", "positive")
	objs, err := testutil.LoadUnstructuredFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (API001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].RemediationDetail != nil {
		t.Errorf("RemediationDetail = %+v, want nil for PodSecurityPolicy (no direct apiVersion replacement)", fs[0].RemediationDetail)
	}
}

func TestAPI001_RemediationDetail_ManifestVariantUsesSourcePath(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "manifest-repo"))
	if err != nil {
		t.Fatalf("resolving fixture repo path: %v", err)
	}
	dep := apicatalog.Deprecated[1] // extensions/v1beta1 Deployment -> apps/v1
	sc := &ScanContext{
		K8s: &k8s.Snapshot{Errors: map[string]error{}},
		Manifests: &manifest.Snapshot{DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{{
			DeprecatedAPI: dep, Namespace: "payments", Name: "legacy-app", SourcePath: filepath.Join(repo, "raw", "deployment.yaml"),
		}}},
	}
	fs, err := (API001{}).Evaluate(sc, "1.34")
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
	wantVerify := fmt.Sprintf("kubepreflight scan --manifests '%s' --target-version '1.34'", filepath.Join(repo, "raw"))
	if rd.VerifyCommand != wantVerify {
		t.Errorf("VerifyCommand = %q, want %q", rd.VerifyCommand, wantVerify)
	}
	if rd.AffectedFile == "" {
		t.Error("AffectedFile is empty, want the manifest source path")
	}
}

// --- Catalog coverage added while auditing internal/apicatalog/catalog.go
// against the official Kubernetes Deprecated API Migration Guide
// (kubernetes.io/docs/reference/using-api/deprecation-guide/). Builds
// k8s.Snapshot.DeprecatedAPIUsage directly, the same way
// TestAPI001_LiveAndManifestPlanes_MergeWithBothOccurrences above already
// does, rather than round-tripping through unstructured fixtures — no new
// fixture files needed for a bare apiVersion/kind/name case.

// findDeprecatedAPI looks up a catalog entry by Group/Version/Kind instead
// of a positional index, so this test suite doesn't care where in the
// slice the entry lives (unlike the handful of pre-existing tests that do
// index positionally into apicatalog.Deprecated[0]/[1] — see the comment
// on those and on the append-only insertion in catalog.go).
func findDeprecatedAPI(t *testing.T, group, version, kind string) apicatalog.DeprecatedAPI {
	t.Helper()
	for _, d := range apicatalog.Deprecated {
		if d.Group == group && d.Version == version && d.Kind == kind {
			return d
		}
	}
	t.Fatalf("no apicatalog.Deprecated entry for %s/%s %s", group, version, kind)
	return apicatalog.DeprecatedAPI{}
}

func TestAPI001_CatalogCoverage_KnownRemovals(t *testing.T) {
	cases := []struct {
		name          string
		group         string
		version       string
		kind          string
		targetVersion string
		wantFinding   bool
	}{
		// The two entries this PR adds to the catalog.
		{"extensions/v1beta1 PodSecurityPolicy removed 1.16", "extensions", "v1beta1", "PodSecurityPolicy", "1.34", true},
		{"storage.k8s.io/v1beta1 CSIStorageCapacity removed 1.27, targeting past removal", "storage.k8s.io", "v1beta1", "CSIStorageCapacity", "1.27", true},
		{"storage.k8s.io/v1beta1 CSIStorageCapacity, targeting before removal", "storage.k8s.io", "v1beta1", "CSIStorageCapacity", "1.26", false},

		// Explicitly requested regression coverage for entries that were
		// already in the catalog but had no dedicated test proving API-001
		// actually detects them.
		{"flowcontrol.apiserver.k8s.io/v1beta3 FlowSchema removed 1.32", "flowcontrol.apiserver.k8s.io", "v1beta3", "FlowSchema", "1.32", true},
		{"flowcontrol.apiserver.k8s.io/v1beta3 FlowSchema, targeting before removal", "flowcontrol.apiserver.k8s.io", "v1beta3", "FlowSchema", "1.31", false},
		{"flowcontrol.apiserver.k8s.io/v1beta2 PriorityLevelConfiguration removed 1.29", "flowcontrol.apiserver.k8s.io", "v1beta2", "PriorityLevelConfiguration", "1.29", true},
		{"flowcontrol.apiserver.k8s.io/v1beta2 PriorityLevelConfiguration, targeting before removal", "flowcontrol.apiserver.k8s.io", "v1beta2", "PriorityLevelConfiguration", "1.28", false},
		{"node.k8s.io/v1beta1 RuntimeClass removed 1.25", "node.k8s.io", "v1beta1", "RuntimeClass", "1.25", true},
		{"node.k8s.io/v1beta1 RuntimeClass, targeting before removal", "node.k8s.io", "v1beta1", "RuntimeClass", "1.24", false},
		{"networking.k8s.io/v1beta1 IngressClass removed 1.22", "networking.k8s.io", "v1beta1", "IngressClass", "1.22", true},
		{"networking.k8s.io/v1beta1 IngressClass, targeting before removal", "networking.k8s.io", "v1beta1", "IngressClass", "1.21", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dep := findDeprecatedAPI(t, tc.group, tc.version, tc.kind)
			sc := &ScanContext{K8s: &k8s.Snapshot{
				Errors: map[string]error{},
				DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
					{DeprecatedAPI: dep, Name: "test-object", UID: "test-uid"},
				},
			}}

			fs, err := (API001{}).Evaluate(sc, tc.targetVersion)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if tc.wantFinding && len(fs) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
			}
			if !tc.wantFinding && len(fs) != 0 {
				t.Fatalf("got %d findings, want 0 (target version before removal must not fire): %+v", len(fs), fs)
			}
			if tc.wantFinding && fs[0].Resources[0].Kind != tc.kind {
				t.Errorf("Resources[0].Kind = %q, want %q", fs[0].Resources[0].Kind, tc.kind)
			}
		})
	}
}

// TestAPI001_LiveEvents_SuppressedEntirely guards the ephemeral-object
// scope decision: live Event objects at a removed API version produce no
// finding at all — not even Info. Nobody hand-authors or migrates an
// Event; it self-expires within about an hour and a real cluster can have
// hundreds at once, so flagging each one individually is noise, not
// signal.
func TestAPI001_LiveEvents_SuppressedEntirely(t *testing.T) {
	dep := findDeprecatedAPI(t, "events.k8s.io", "v1beta1", "Event")
	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Namespace: "default", Name: "some-pod.abcdef123456", UID: "evt-uid-1"},
			{DeprecatedAPI: dep, Namespace: "default", Name: "some-pod.abcdef654321", UID: "evt-uid-2"},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings for live Event objects, want 0 (suppressed entirely): %+v", len(fs), fs)
	}
}

// TestAPI001_ManifestEvents_StillFireAsBlocker guards against
// over-suppressing: the ephemeral-object exclusion is scoped to the K8s
// (live) plane only, in Evaluate's loop structure — a manifest-plane Event
// (unusual, but the collector doesn't forbid it) is inherently
// user-authored YAML, so it must still be reported normally.
func TestAPI001_ManifestEvents_StillFireAsBlocker(t *testing.T) {
	dep := findDeprecatedAPI(t, "events.k8s.io", "v1beta1", "Event")
	sc := &ScanContext{Manifests: &manifest.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Namespace: "default", Name: "manual-event", SourcePath: "manifests/event.yaml"},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("manifest-plane Event = %+v, want exactly one Blocker finding (manifest plane is unaffected by the live-only Event suppression)", fs)
	}
}

// TestAPI001_AutoManagedFlowSchema_DowngradesToInfo guards the second
// ephemeral-object decision: a kube-apiserver bootstrap FlowSchema/
// PriorityLevelConfiguration default (marked with the real
// apf.kubernetes.io/autoupdate-spec annotation, confirmed against a live
// cluster) is still reported — unlike Events — but as Info with copy that
// says there's usually nothing to do, since the API server owns and
// recreates these itself.
func TestAPI001_AutoManagedFlowSchema_DowngradesToInfo(t *testing.T) {
	dep := findDeprecatedAPI(t, "flowcontrol.apiserver.k8s.io", "v1beta1", "FlowSchema")
	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Name: "exempt", UID: "fs-uid-1", AutoManaged: true},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Severity != findings.SeverityInfo {
		t.Errorf("Severity = %q, want Info for a kube-apiserver-managed default", f.Severity)
	}
	if !strings.Contains(f.Message, "usually no direct user action") {
		t.Errorf("Message = %q, want it to say there's usually no direct user action", f.Message)
	}
	if f.RemediationDetail != nil {
		t.Errorf("RemediationDetail = %+v, want nil (no diff to show for an object the reader doesn't edit)", f.RemediationDetail)
	}
}

// TestAPI001_UserCreatedFlowSchema_StillFiresAsBlocker is the regression
// guard against over-suppressing: a FlowSchema/PriorityLevelConfiguration
// WITHOUT the apf.kubernetes.io/autoupdate-spec annotation is a real,
// user-owned object (AutoManaged defaults to false, matching what the
// collector observes for anything kube-apiserver doesn't itself own) and
// must keep firing exactly as it always has — full Blocker, real
// migration remediation.
func TestAPI001_UserCreatedFlowSchema_StillFiresAsBlocker(t *testing.T) {
	dep := findDeprecatedAPI(t, "flowcontrol.apiserver.k8s.io", "v1beta1", "PriorityLevelConfiguration")
	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Name: "team-custom-priority", UID: "plc-uid-1", AutoManaged: false},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("user-created PriorityLevelConfiguration = %+v, want exactly one Blocker finding", fs)
	}
	if fs[0].RemediationDetail == nil {
		t.Errorf("RemediationDetail = nil, want a real diff for a user-owned object with a direct apiVersion swap")
	}
}

// TestAPI001_DemoSeededObjects_UnaffectedByEphemeralFiltering is a direct
// regression guard for the demo/README-documented seeded fixtures (PSP,
// PDB) — proving the ephemeral-object exclusion is scoped to exactly
// events.k8s.io and auto-managed flowcontrol objects, nothing broader.
func TestAPI001_DemoSeededObjects_UnaffectedByEphemeralFiltering(t *testing.T) {
	psp := findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy")
	pdb := findDeprecatedAPI(t, "policy", "v1beta1", "PodDisruptionBudget")
	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: psp, Name: "demo-restricted", UID: "psp-uid"},
			{DeprecatedAPI: pdb, Namespace: "demo", Name: "shared-app-pdb-a", UID: "pdb-uid"},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings for demo's seeded PSP+PDB, want 2 (unaffected by ephemeral-object filtering): %+v", len(fs), fs)
	}
	for _, f := range fs {
		if f.Severity != findings.SeverityBlocker {
			t.Errorf("demo seeded object %s severity = %q, want Blocker", f.Resources[0].Name, f.Severity)
		}
	}
}

// TestAPI001_ControllerManagedEndpointSlice_DowngradesToInfo guards the
// EndpointSlice ephemeral-object decision: an EndpointSlice the built-in
// EndpointSlice controller created (marked with the real
// endpointslice.kubernetes.io/managed-by label, confirmed against a live
// cluster) is Info, not Blocker, with remediation that says there's
// usually nothing to do — the controller keeps recreating it against its
// owning Service.
func TestAPI001_ControllerManagedEndpointSlice_DowngradesToInfo(t *testing.T) {
	dep := findDeprecatedAPI(t, "discovery.k8s.io", "v1beta1", "EndpointSlice")
	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Namespace: "payments", Name: "checkout-svc-abc12", UID: "eps-uid-1", AutoManaged: true},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Severity != findings.SeverityInfo {
		t.Errorf("Severity = %q, want Info for a controller-managed EndpointSlice", f.Severity)
	}
	if !strings.Contains(f.Message, "usually no direct user action") {
		t.Errorf("Message = %q, want it to say there's usually no direct user action", f.Message)
	}
	if f.RemediationDetail != nil {
		t.Errorf("RemediationDetail = %+v, want nil (no diff to show for an object the reader doesn't edit)", f.RemediationDetail)
	}
}

// TestAPI001_UnmanagedEndpointSlice_StillFiresAsBlocker is the regression
// guard against over-suppressing: an EndpointSlice WITHOUT the
// endpointslice.kubernetes.io/managed-by label (AutoManaged defaults to
// false) — matching the one real exception observed, the default/
// kubernetes Service's own EndpointSlice, which some clusters create
// without going through the standard controller — must keep firing as a
// full Blocker exactly as before, since there's no confirmed signal it's
// safe to downgrade.
func TestAPI001_UnmanagedEndpointSlice_StillFiresAsBlocker(t *testing.T) {
	dep := findDeprecatedAPI(t, "discovery.k8s.io", "v1beta1", "EndpointSlice")
	sc := &ScanContext{K8s: &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Namespace: "default", Name: "kubernetes", UID: "eps-uid-2", AutoManaged: false},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("unmanaged EndpointSlice = %+v, want exactly one Blocker finding", fs)
	}
	if fs[0].RemediationDetail == nil {
		t.Errorf("RemediationDetail = nil, want a real diff for a Blocker-severity EndpointSlice finding")
	}
}

// TestAPI001_ManifestEndpointSlice_UnaffectedByAutoManagedCheck guards
// scope: the AutoManaged downgrade only applies to the live plane
// (isControllerManagedEndpointSlice is only checked in Evaluate's sc.K8s
// loop) — a manifest-plane EndpointSlice (unusual, but not forbidden) is
// inherently user-authored YAML and must fire as a normal Blocker.
func TestAPI001_ManifestEndpointSlice_UnaffectedByAutoManagedCheck(t *testing.T) {
	dep := findDeprecatedAPI(t, "discovery.k8s.io", "v1beta1", "EndpointSlice")
	sc := &ScanContext{Manifests: &manifest.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []manifest.DeprecatedAPIObject{
			{DeprecatedAPI: dep, Namespace: "payments", Name: "manual-eps", SourcePath: "manifests/endpointslice.yaml"},
		},
	}}

	fs, err := (API001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("manifest-plane EndpointSlice = %+v, want exactly one Blocker finding", fs)
	}
}

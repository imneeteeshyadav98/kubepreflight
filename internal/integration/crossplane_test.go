package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"kubepreflight/internal/apicatalog"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
	"kubepreflight/internal/report"
	"kubepreflight/internal/rules"
)

func TestCrossPlaneAPI001_CombinedFindingsJSON(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", "testdata", "manifest-repo"))
	if err != nil {
		t.Fatalf("resolving manifest fixture path: %v", err)
	}
	manifestSnap, err := manifest.NewCollector([]string{filepath.Join(repo, "raw")}, nil).Collect(context.Background(), time.Second)
	if err != nil {
		t.Fatalf("collecting manifests: %v", err)
	}
	liveSnap := &k8s.Snapshot{
		Errors: map[string]error{},
		DeprecatedAPIUsage: []k8s.DeprecatedAPIObject{{
			DeprecatedAPI: findDeprecatedAPI(t, "policy", "v1beta1", "PodSecurityPolicy"),
			Name:          "manifest-restricted",
			UID:           "live-uid-1",
		}},
	}

	fs, err := (rules.API001{}).Evaluate(&rules.ScanContext{K8s: liveSnap, Manifests: manifestSnap}, "1.34")
	if err != nil {
		t.Fatalf("API-001 Evaluate: %v", err)
	}
	rpt := findings.NewReport("1.34", "fixture-cluster", "", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC), fs)

	var buf bytes.Buffer
	if err := report.WriteJSON(rpt, &buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var decoded findings.Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding findings.json: %v", err)
	}
	if len(decoded.Findings) != 1 || len(decoded.Findings[0].Resources) != 2 {
		t.Fatalf("combined findings = %+v, want one finding with two occurrences", decoded.Findings)
	}
	if len(decoded.Assumptions) != 1 || decoded.Assumptions[0] != findings.CrossPlaneManifestAssumption {
		t.Fatalf("assumptions = %v, want cross-plane target-cluster assumption", decoded.Assumptions)
	}

	t.Logf("combined findings.json:\n%s", buf.String())
}

// findDeprecatedAPI looks up a catalog entry by Group/Version/Kind rather
// than a positional index — apicatalog.Deprecated is derived from the
// versioned catalog's deterministic sort order, not hand-authored
// declaration order, so no test may index into it positionally.
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

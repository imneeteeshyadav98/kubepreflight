package integration

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

func TestLargeScaleReportsRemainValidAcrossFormats(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("large")
	if !ok {
		t.Fatal("large scale scenario missing")
	}
	rpt := mustScaleReport(t, cfg)
	if len(rpt.Findings) < 1000 {
		t.Fatalf("large scale report has %d findings, want at least 1000 to exercise large-output paths", len(rpt.Findings))
	}

	var jsonOut bytes.Buffer
	if err := report.WriteJSON(rpt, &jsonOut); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var decoded findings.Report
	if err := json.Unmarshal(jsonOut.Bytes(), &decoded); err != nil {
		t.Fatalf("large findings JSON is invalid: %v", err)
	}
	if decoded.SchemaVersion != findings.SchemaVersion || len(decoded.Findings) != len(rpt.Findings) {
		t.Fatalf("decoded JSON schema/findings = %q/%d, want %q/%d", decoded.SchemaVersion, len(decoded.Findings), findings.SchemaVersion, len(rpt.Findings))
	}

	var mdOut bytes.Buffer
	if err := report.WriteMarkdown(rpt, &mdOut); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	for _, want := range []string{"# KubePreflight Scan Report", "| **Result** |", "## Evidence Appendix"} {
		if !strings.Contains(mdOut.String(), want) {
			t.Fatalf("large Markdown missing %q", want)
		}
	}

	var htmlOut bytes.Buffer
	if err := report.WriteHTML(rpt, &htmlOut); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	html := htmlOut.String()
	if !strings.Contains(strings.ToLower(html), "<!doctype html>") {
		t.Fatal("large HTML missing doctype")
	}
	for _, want := range []string{"KubePreflight Scan Report", "data-finding=\"true\""} {
		if !strings.Contains(html, want) {
			t.Fatalf("large HTML missing %q", want)
		}
	}
}

func TestLargeScaleComparisonIsValidAndStable(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("large")
	if !ok {
		t.Fatal("large scale scenario missing")
	}
	rpt := mustScaleReport(t, cfg)
	cmp, err := comparison.Compare(rpt, rpt)
	if err != nil {
		t.Fatalf("Compare large report with itself: %v", err)
	}
	if cmp.SchemaVersion != comparison.SchemaVersion {
		t.Fatalf("comparison schemaVersion = %q, want %q", cmp.SchemaVersion, comparison.SchemaVersion)
	}
	if cmp.Summary.Unchanged != len(rpt.Findings) || len(cmp.New) != 0 || len(cmp.Resolved) != 0 || len(cmp.Changed) != 0 {
		t.Fatalf("large self-comparison summary = %+v, buckets new/resolved/changed=%d/%d/%d, want all findings unchanged", cmp.Summary, len(cmp.New), len(cmp.Resolved), len(cmp.Changed))
	}
	if len(cmp.Unchanged) != len(rpt.Findings) {
		t.Fatalf("unchanged bucket = %d, want %d", len(cmp.Unchanged), len(rpt.Findings))
	}
}

func TestMediumScalePipelineKeepsFindingIdentitiesStable(t *testing.T) {
	cfg, ok := testutil.ScaleScenarioConfig("medium")
	if !ok {
		t.Fatal("medium scale scenario missing")
	}
	fixture, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		t.Fatalf("GenerateScaleFixture: %v", err)
	}
	sc := &rules.ScanContext{K8s: fixture.Snapshot}
	first, err := rules.NewDefaultRegistry().RunAll(sc, scaleBenchmarkTargetVersion)
	if err != nil {
		t.Fatalf("RunAll first: %v", err)
	}
	second, err := rules.NewDefaultRegistry().RunAll(sc, scaleBenchmarkTargetVersion)
	if err != nil {
		t.Fatalf("RunAll second: %v", err)
	}
	firstReport := findings.NewReport(scaleBenchmarkTargetVersion, "scale-medium", "", time.Unix(0, 0).UTC(), first)
	secondReport := findings.NewReport(scaleBenchmarkTargetVersion, "scale-medium", "", time.Unix(0, 0).UTC(), second)
	cmp, err := comparison.Compare(firstReport, secondReport)
	if err != nil {
		t.Fatalf("Compare repeated medium reports: %v", err)
	}
	if cmp.Summary.New != 0 || cmp.Summary.Resolved != 0 || cmp.Summary.Changed != 0 || cmp.Summary.Unchanged != len(firstReport.Findings) {
		t.Fatalf("repeated medium pipeline comparison = %+v, want every finding unchanged", cmp.Summary)
	}
}

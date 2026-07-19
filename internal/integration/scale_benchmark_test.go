package integration

import (
	"io"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/comparison"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
	"github.com/imneeteeshyadav98/kubepreflight/internal/report"
	"github.com/imneeteeshyadav98/kubepreflight/internal/rules"
	"github.com/imneeteeshyadav98/kubepreflight/internal/testutil"
)

const scaleBenchmarkTargetVersion = "1.34"

func BenchmarkScaleFixtureGeneration(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				fixture, err := testutil.GenerateScaleFixture(cfg)
				if err != nil {
					b.Fatalf("GenerateScaleFixture: %v", err)
				}
				if fixture.Snapshot == nil {
					b.Fatal("GenerateScaleFixture returned nil snapshot")
				}
			}
		})
	}
}

func BenchmarkScaleRuleEvaluation(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		fixture := mustScaleFixture(b, cfg)
		sc := &rules.ScanContext{K8s: fixture.Snapshot}
		registry := rules.NewDefaultRegistry()
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fs, err := registry.RunAll(sc, scaleBenchmarkTargetVersion)
				if err != nil {
					b.Fatalf("RunAll: %v", err)
				}
				if len(fs) == 0 {
					b.Fatal("RunAll returned no findings")
				}
			}
		})
	}
}

func BenchmarkScaleReportConstruction(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		fs := mustScaleFindings(b, cfg)
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rpt := findings.NewReport(scaleBenchmarkTargetVersion, "scale-"+cfg.Name, "", time.Unix(0, 0).UTC(), fs)
				if rpt.UpgradeReadiness == nil {
					b.Fatal("NewReport produced nil UpgradeReadiness")
				}
			}
		})
	}
}

func BenchmarkScaleReportJSON(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		rpt := mustScaleReport(b, cfg)
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := report.WriteJSON(rpt, io.Discard); err != nil {
					b.Fatalf("WriteJSON: %v", err)
				}
			}
		})
	}
}

func BenchmarkScaleReportMarkdown(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		rpt := mustScaleReport(b, cfg)
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := report.WriteMarkdown(rpt, io.Discard); err != nil {
					b.Fatalf("WriteMarkdown: %v", err)
				}
			}
		})
	}
}

func BenchmarkScaleReportHTML(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		rpt := mustScaleReport(b, cfg)
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := report.WriteHTML(rpt, io.Discard); err != nil {
					b.Fatalf("WriteHTML: %v", err)
				}
			}
		})
	}
}

func BenchmarkScaleComparison(b *testing.B) {
	for _, cfg := range testutil.ScaleScenarioConfigs() {
		rpt := mustScaleReport(b, cfg)
		b.Run(cfg.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cmp, err := comparison.Compare(rpt, rpt)
				if err != nil {
					b.Fatalf("Compare: %v", err)
				}
				if cmp.Summary.Unchanged == 0 {
					b.Fatal("Compare returned no unchanged findings")
				}
			}
		})
	}
}

func mustScaleFixture(tb testing.TB, cfg testutil.ScaleFixtureConfig) *testutil.ScaleFixture {
	tb.Helper()
	fixture, err := testutil.GenerateScaleFixture(cfg)
	if err != nil {
		tb.Fatalf("GenerateScaleFixture(%s): %v", cfg.Name, err)
	}
	return fixture
}

func mustScaleFindings(tb testing.TB, cfg testutil.ScaleFixtureConfig) []findings.Finding {
	tb.Helper()
	fixture := mustScaleFixture(tb, cfg)
	fs, err := rules.NewDefaultRegistry().RunAll(&rules.ScanContext{K8s: fixture.Snapshot}, scaleBenchmarkTargetVersion)
	if err != nil {
		tb.Fatalf("RunAll(%s): %v", cfg.Name, err)
	}
	return fs
}

func mustScaleReport(tb testing.TB, cfg testutil.ScaleFixtureConfig) *findings.Report {
	tb.Helper()
	fs := mustScaleFindings(tb, cfg)
	return findings.NewReport(scaleBenchmarkTargetVersion, "scale-"+cfg.Name, "", time.Unix(0, 0).UTC(), fs)
}

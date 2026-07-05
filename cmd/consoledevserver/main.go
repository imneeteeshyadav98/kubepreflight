// Command consoledevserver starts the real local report server
// (internal/reportserver, the same code path `kubepreflight scan` uses)
// against an existing report output directory, so the Console browser
// smoke test (web/tests/browser_smoke.py) can exercise the actual
// embedded-Console mount at /console/ instead of a stand-in static file
// server. Not part of the public CLI — not built or shipped by the
// Dockerfile, which only compiles ./cmd/kubepreflight.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/report"
	"kubepreflight/internal/reportserver"
)

func main() {
	dir := flag.String("dir", ".", "directory containing report.html and the findings JSON")
	findingsName := flag.String("findings", "findings.json", "findings file name within dir")
	listen := flag.String("listen", "127.0.0.1:0", "listen address")
	synthetic := flag.Bool("synthetic", false, "ignore --dir/--findings and serve a freshly generated, cluster-independent findings.json/report.html (see writeSyntheticFixture)")
	refresh := flag.Bool("refresh", false, "refresh findings.json/report.md/report.html in --dir using the current schemas and renderers, then exit")
	flag.Parse()
	if *refresh {
		if err := refreshFixture(*dir, *findingsName); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	outputDir := *dir
	findingsPath := *findingsName
	if *synthetic {
		tempDir, err := os.MkdirTemp("", "kubepreflight-synthetic-fixture-")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer os.RemoveAll(tempDir)
		if err := writeSyntheticFixture(tempDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		outputDir = tempDir
		findingsPath = "findings.json"
	}

	server, err := reportserver.Start(reportserver.Config{Listen: *listen, OutputDir: outputDir, FindingsPath: findingsPath, ServePlan: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("REPORT " + server.ReportURL())
	if consoleURL, ok := server.ConsoleURL(); ok {
		fmt.Println("CONSOLE " + consoleURL)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Wait(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func refreshFixture(dir, findingsName string) error {
	path := filepath.Join(dir, findingsName)
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var rpt findings.Report
	if err := json.Unmarshal(raw, &rpt); err != nil {
		return err
	}
	rpt.SchemaVersion = findings.SchemaVersion
	if rpt.Coverage.Kubernetes.Status == "" {
		rpt.Coverage.Kubernetes.Status = findings.CoverageComplete
	}
	if rpt.Coverage.AWS.Status == "" {
		rpt.Coverage.AWS.Status = findings.CoverageSkipped
	}
	if rpt.Coverage.Manifests.Status == "" {
		rpt.Coverage.Manifests.Status = findings.CoverageSkipped
	}
	writers := []struct {
		name  string
		write func(*findings.Report, io.Writer) error
	}{{findingsName, report.WriteJSON}, {"terminal-output.txt", report.WriteTerminal}, {"report.md", report.WriteMarkdown}, {"report.html", report.WriteHTML}}
	for _, target := range writers {
		file, err := os.Create(filepath.Join(dir, target.name))
		if err != nil {
			return err
		}
		if err := target.write(&rpt, file); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

// writeSyntheticFixture renders findings.json/report.html straight from
// internal/report — no cluster, no manifests directory — so tests that need
// current, up-to-date output (like the browser smoke test's horizontal-
// overflow guard) never depend on a real kind cluster's live state, and
// never go stale the way a committed fixture (demo/sample-output/) can.
// Deliberately includes long resource names, an overlap list, and a long
// remediation command — the exact content shapes that have caused real
// wrap/overflow regressions in report.html and the Console.
func writeSyntheticFixture(dir string) error {
	fs := []findings.Finding{
		{
			RuleID: "PDB-002", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
			Message: `PodDisruptionBudgets "preflight-lab/critical-app-pdb" and "preflight-lab/critical-app-pdb-overlap" select an overlapping set of pods, which is always a misconfiguration under the eviction API`,
			Resources: []findings.ResourceReference{
				findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "preflight-lab", "critical-app-pdb", "uid-pdb-1"),
				findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "preflight-lab", "critical-app-pdb-overlap", "uid-pdb-2"),
			},
			Evidence:    []string{"minAvailable: 1", "currentHealthy: 1", "desiredHealthy: 1", "expectedPods: 3"},
			Remediation: "Inspect both budgets and their owners, then remove a confirmed duplicate or narrow one selector so each pod is selected by at most one PodDisruptionBudget.",
			Fingerprint: "fp-synthetic-pdb-overlap",
		},
		{
			RuleID: "API-001", Severity: findings.SeverityBlocker, Confidence: findings.TierStaticCertain,
			Message:     `PodDisruptionBudget "default/old-pdb-api" (apiVersion policy/v1beta1) in demo-manifests-local/old-api.yaml uses an API version removed in Kubernetes 1.25 — this manifest will fail to apply once the cluster reaches target 1.36`,
			Resources:   []findings.ResourceReference{findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "old-pdb-api", "demo-manifests-local/old-api.yaml")},
			Evidence:    []string{"apiVersion: policy/v1beta1", "removed in: Kubernetes 1.25", "target version: 1.36"},
			Remediation: "Migrate to policy/v1 PodDisruptionBudget before this manifest is ever applied to a cluster at or past 1.25. Update and validate the source manifest against the replacement schema. For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.",
			Fingerprint: "fp-synthetic-api-001",
		},
		{
			RuleID: "WH-002", Severity: findings.SeverityBlocker, Confidence: findings.TierObserved,
			Message:     `ValidatingWebhookConfiguration "dead-fail-closed-webhook" is fail-closed with a catch-all apiGroups/resources scope and zero ready backend endpoints`,
			Resources:   []findings.ResourceReference{findings.LiveResource("ValidatingWebhookConfiguration", findings.ScopeCluster, "", "dead-fail-closed-webhook", "uid-webhook-1")},
			Evidence:    []string{"webhook index: 0", "ready endpoint address count: 0", "failurePolicy: Fail"},
			Remediation: "Restore ready backend endpoints before the upgrade. If API writes are already blocked, use the guarded emergency patch only after confirming this exact webhook entry, then restore failurePolicy: Fail after recovery.",
			Fingerprint: "fp-synthetic-wh-002",
		},
	}
	rpt := findings.NewReport("1.36", "synthetic-fixture", "cluster-only", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), fs)

	findingsFile, err := os.Create(filepath.Join(dir, "findings.json"))
	if err != nil {
		return err
	}
	defer findingsFile.Close()
	if err := report.WriteJSON(rpt, findingsFile); err != nil {
		return err
	}

	reportFile, err := os.Create(filepath.Join(dir, "report.html"))
	if err != nil {
		return err
	}
	defer reportFile.Close()
	return report.WriteHTML(rpt, reportFile)
}

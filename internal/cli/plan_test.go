package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/plan"
)

func TestPlanCommandExposesExpectedFlags(t *testing.T) {
	exitCode := 0
	cmd := newPlanCmd(&exitCode)
	for _, name := range []string{
		"from-version", "to-version", "serve-report", "open-report", "listen", "terminal-output",
		"provider", "cluster-name", "resource-group", "subscription-id", "project", "location",
		"manifests", "helm-chart", "namespace-allowlist",
		"output", "findings-out", "kubeconfig", "context",
	} {
		if flag := cmd.Flags().Lookup(name); flag == nil {
			t.Errorf("plan command has no --%s flag", name)
		}
	}
}

func TestPlanCommandListenDefaultsToFixedPort(t *testing.T) {
	exitCode := 0
	cmd := newPlanCmd(&exitCode)
	flag := cmd.Flags().Lookup("listen")
	if flag == nil {
		t.Fatal("plan command has no --listen flag")
	}
	if flag.DefValue != "127.0.0.1:8080" {
		t.Errorf("--listen default = %q, want 127.0.0.1:8080", flag.DefValue)
	}
}

func TestPlanCommandRequiresToVersionBeforeClusterAccess(t *testing.T) {
	exitCode := 0
	cmd := newPlanCmd(&exitCode)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("plan with no --to-version succeeded, want validation error")
	}
}

func TestPlanCommandValidatesFlagsBeforeClusterAccess(t *testing.T) {
	for _, args := range [][]string{
		{"--to-version", "1.36", "--terminal-output", "verbose"},
		{"--to-version", "1.36", "--serve-report", "sometimes"},
		{"--to-version", "1.36", "--serve-report", "never", "--open-report"},
		{"--to-version", "1.36", "--output", "yaml"},
		{"--to-version", "1.36", "--provider", "gcp"},
		{"--to-version", "1.36", "--provider", "eks"},                                                               // missing --cluster-name
		{"--to-version", "1.36", "--provider", "aks", "--cluster-name", "x"},                                        // missing --resource-group
		{"--to-version", "1.36", "--provider", "gke", "--cluster-name", "x", "--project", "p"},                      // missing --location
		{"--to-version", "1.36", "--provider", "aks", "--cluster-name", "x", "--resource-group", "rg"},              // valid flags, but not implemented yet
		{"--to-version", "1.36", "--provider", "gke", "--cluster-name", "x", "--project", "p", "--location", "us1"}, // valid flags, but not implemented yet
		{"--to-version", "garbage"},
		{"--to-version", "1.36", "--from-version", "garbage"},
	} {
		exitCode := 0
		cmd := newPlanCmd(&exitCode)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("plan %v succeeded, want validation error before any cluster access", args)
		}
	}
}

func TestIsManifestOnlyFinding(t *testing.T) {
	manifestOnly := findings.Finding{
		Resources: []findings.ResourceReference{
			findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "x", "path.yaml"),
		},
	}
	if !isManifestOnlyFinding(manifestOnly) {
		t.Error("isManifestOnlyFinding(manifest-only) = false, want true")
	}

	liveOnly := findings.Finding{
		Resources: []findings.ResourceReference{
			findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "x", "uid-1"),
		},
	}
	if isManifestOnlyFinding(liveOnly) {
		t.Error("isManifestOnlyFinding(live-only) = true, want false")
	}

	crossPlane := findings.Finding{
		Resources: []findings.ResourceReference{
			findings.LiveResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "x", "uid-1"),
			findings.ManifestResource("PodDisruptionBudget", findings.ScopeNamespaced, "default", "x", "path.yaml"),
		},
	}
	if isManifestOnlyFinding(crossPlane) {
		t.Error("isManifestOnlyFinding(cross-plane merged) = true, want false — must not project a live-linked finding forward")
	}

	empty := findings.Finding{}
	if isManifestOnlyFinding(empty) {
		t.Error("isManifestOnlyFinding(no resources) = true, want false")
	}
}

func TestBuildRecommendedScanCommand(t *testing.T) {
	got := buildRecommendedScanCommand("1.31", "eks", "my-cluster", []string{"./k8s"}, []string{"./chart"}, []string{"payments", "platform"})
	for _, want := range []string{
		"kubepreflight scan",
		"--target-version 1.31",
		"--provider eks",
		"--cluster-name my-cluster",
		"--manifests ./k8s",
		"--helm-chart ./chart",
		"--namespace-allowlist payments,platform",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("buildRecommendedScanCommand() = %q, want it to contain %q", got, want)
		}
	}
}

func TestBuildRecommendedScanCommand_ClusterOnlyOmitsProviderFlags(t *testing.T) {
	got := buildRecommendedScanCommand("1.31", "", "", nil, nil, nil)
	for _, unwanted := range []string{"--provider", "--cluster-name", "--manifests", "--helm-chart", "--namespace-allowlist"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("buildRecommendedScanCommand() = %q, should not contain %q when unset", got, unwanted)
		}
	}
}

// TestWritePlanHTMLFile_WritesPlanAwareReport guards the plan command's
// report.html wiring: writePlanHTMLFile (used only for the "report.html"
// target in the plan command's write loop) must produce the plan-aware
// Upgrade Path renderer's output, not a plain hop-1-only scan report.
// Driving the full plan command needs a live cluster (unavailable here),
// so this tests the wiring function directly, matching this session's
// established offline-fixture pattern.
func TestWritePlanHTMLFile_WritesPlanAwareReport(t *testing.T) {
	hop1 := findings.NewReport("1.30", "test-cluster", "eks", time.Now(), nil)
	hops := []plan.Hop{{Index: 1, From: "1.29", To: "1.30"}}
	planReport, err := plan.BuildPlan("test-cluster", "eks", "1.29", "explicit-flag", "1.30", hops, hop1, nil, time.Now())
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	path := filepath.Join(t.TempDir(), "report.html")
	if err := writePlanHTMLFile(path, planReport); err != nil {
		t.Fatalf("writePlanHTMLFile: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if !strings.Contains(string(raw), "Upgrade Path (") {
		t.Errorf("report.html written by writePlanHTMLFile is missing the Upgrade Path section: %s", raw)
	}
}

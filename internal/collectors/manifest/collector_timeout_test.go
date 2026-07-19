package manifest_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/manifest"
)

// writeHangingHelmStub puts a fake "helm" executable that sleeps far longer
// than any timeout used here on PATH for the duration of the test,
// simulating a hung `helm template` invocation (e.g. blocked on a slow or
// unreachable chart-dependency registry) -- a real external process, not a
// mock, the same testing philosophy PR #95 used against a real
// black-holed API server address for the k8s collector.
func writeHangingHelmStub(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "helm")
	if runtime.GOOS == "windows" {
		t.Skip("hanging-helm-stub test relies on a POSIX shell script named exactly \"helm\"")
	}
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 300\n"), 0o755); err != nil {
		t.Fatalf("writing fake helm stub: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCollector_ScanHelmChart_HangingHelmBoundedByTimeout(t *testing.T) {
	writeHangingHelmStub(t)

	repo := fixtureRepoDir(t)
	chartPath := filepath.Join(repo, "chart")
	c := manifest.NewCollector(nil, []manifest.HelmChart{{Path: chartPath, ReleaseName: "legacy-app"}})

	start := time.Now()
	snap, err := c.Collect(context.Background(), 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if elapsed > 8*time.Second {
		t.Fatalf("Collect took %s, want it bounded near the 50ms timeout plus the 5s WaitDelay grace period (helm stub sleeps 300s if not killed)", elapsed)
	}

	key := "helm-chart:" + chartPath
	if _, ok := snap.Errors[key]; !ok {
		t.Fatalf("Errors[%q] not set, want the timed-out helm render recorded", key)
	}
}

func TestCollector_ScanHelmChart_OneHungChartDoesNotStarveTheRest(t *testing.T) {
	writeHangingHelmStub(t)

	repo := fixtureRepoDir(t)
	rawDir := filepath.Join(repo, "raw")
	chartPath := filepath.Join(repo, "chart")
	c := manifest.NewCollector([]string{rawDir}, []manifest.HelmChart{{Path: chartPath, ReleaseName: "legacy-app"}})

	start := time.Now()
	snap, err := c.Collect(context.Background(), 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if elapsed > 8*time.Second {
		t.Fatalf("Collect took %s, want it bounded near the 50ms timeout plus the 5s WaitDelay grace period", elapsed)
	}
	// The raw manifest directory (scanned before the hung Helm chart, see
	// Collect's ordering) must still have found its deprecated-API match --
	// a hung chart render must not prevent it.
	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Errorf("got %d deprecated API matches, want 1 from the raw manifest dir despite the hung chart: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}
}

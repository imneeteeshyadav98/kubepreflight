package manifest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kubepreflight/internal/collectors/manifest"
)

func fixtureRepoDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "testdata", "manifest-repo"))
	if err != nil {
		t.Fatalf("resolving fixture repo path: %v", err)
	}
	return dir
}

func TestCollector_ScanDir_FindsDeprecatedAPIAndSkipsCurrent(t *testing.T) {
	repo := fixtureRepoDir(t)
	c := manifest.NewCollector([]string{filepath.Join(repo, "raw")}, nil)

	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("got %d deprecated API matches, want 1 (only the PSP, not the current Deployment): %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	obj := snap.DeprecatedAPIUsage[0]
	if obj.Kind != "PodSecurityPolicy" || obj.Name != "manifest-restricted" {
		t.Errorf("DeprecatedAPIUsage[0] = %+v, want PodSecurityPolicy/manifest-restricted", obj)
	}
	if filepath.Base(obj.SourcePath) != "psp.yaml" {
		t.Errorf("SourcePath = %q, want to end in psp.yaml", obj.SourcePath)
	}
	// The regression this guards: an absolute --manifests root (fixtureRepoDir
	// always returns an absolute path) must never leak into SourcePath — it
	// should be relative to the scanned root, not the operator's local
	// absolute directory layout.
	if obj.SourcePath != "psp.yaml" {
		t.Errorf("SourcePath = %q, want exactly %q (relative to the scanned root, no absolute prefix)", obj.SourcePath, "psp.yaml")
	}
	if filepath.IsAbs(obj.SourcePath) {
		t.Errorf("SourcePath = %q, must not be an absolute path", obj.SourcePath)
	}
}

func TestCollector_ScanDir_RecordsStructuredWorkloadPodSpecs(t *testing.T) {
	dir := t.TempDir()
	raw := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: apps
  name: api
spec:
  template:
    spec:
      nodeSelector:
        node-role.kubernetes.io/master: ""
---
apiVersion: batch/v1
kind: CronJob
metadata:
  namespace: batch
  name: cleanup
spec:
  jobTemplate:
    spec:
      template:
        spec:
          tolerations:
          - key: node-role.kubernetes.io/master
            operator: Exists
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ignored
data:
  key: node-role.kubernetes.io/master
`)
	if err := os.WriteFile(filepath.Join(dir, "workloads.yaml"), raw, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	c := manifest.NewCollector([]string{dir}, nil)
	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}
	if len(snap.Workloads) != 2 {
		t.Fatalf("got %d workloads, want Deployment and CronJob only: %+v", len(snap.Workloads), snap.Workloads)
	}

	byKind := map[string]manifest.WorkloadObject{}
	for _, obj := range snap.Workloads {
		byKind[obj.Kind] = obj
		if obj.SourcePath != "workloads.yaml" {
			t.Errorf("%s SourcePath = %q, want workloads.yaml", obj.Kind, obj.SourcePath)
		}
	}
	if byKind["Deployment"].PodSpecPath != "spec.template.spec" {
		t.Errorf("Deployment PodSpecPath = %q", byKind["Deployment"].PodSpecPath)
	}
	if byKind["CronJob"].PodSpecPath != "spec.jobTemplate.spec.template.spec" {
		t.Errorf("CronJob PodSpecPath = %q", byKind["CronJob"].PodSpecPath)
	}
}

// TestCollector_ScanDir_RelativeRootStillProducesSensibleSourcePath guards
// that a relative --manifests root (the common case — users rarely type an
// absolute path by hand) still resolves correctly rather than only working
// for the absolute-root case.
func TestCollector_ScanDir_RelativeRootStillProducesSensibleSourcePath(t *testing.T) {
	abs := fixtureRepoDir(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	rel, err := filepath.Rel(cwd, filepath.Join(abs, "raw"))
	if err != nil {
		t.Fatalf("computing relative fixture path: %v", err)
	}

	c := manifest.NewCollector([]string{rel}, nil)
	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("got %d deprecated API matches, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}
	if got := snap.DeprecatedAPIUsage[0].SourcePath; got != "psp.yaml" {
		t.Errorf("SourcePath = %q, want %q even when --manifests was given as a relative path", got, "psp.yaml")
	}
}

// TestCollector_ScanDir_SingleFileRootUsesBasenameOnly guards the
// single-file --manifests case: pointing --manifests directly at one file
// (rather than a directory) must not expose that file's full absolute
// parent directory in SourcePath.
func TestCollector_ScanDir_SingleFileRootUsesBasenameOnly(t *testing.T) {
	repo := fixtureRepoDir(t)
	singleFile := filepath.Join(repo, "raw", "psp.yaml")
	c := manifest.NewCollector([]string{singleFile}, nil)

	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("got %d deprecated API matches, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}
	if got := snap.DeprecatedAPIUsage[0].SourcePath; got != "psp.yaml" {
		t.Errorf("SourcePath = %q, want just the basename %q with no absolute parent directory", got, "psp.yaml")
	}
}

func TestCollector_ScanHelmChart_RendersAndFindsDeprecatedAPI(t *testing.T) {
	repo := fixtureRepoDir(t)
	chart := manifest.HelmChart{Path: filepath.Join(repo, "chart"), ReleaseName: "legacy-app"}
	c := manifest.NewCollector(nil, []manifest.HelmChart{chart})

	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.Errors) != 0 {
		t.Fatalf("unexpected collector errors: %v", snap.Errors)
	}

	if len(snap.DeprecatedAPIUsage) != 1 {
		t.Fatalf("got %d deprecated API matches from the chart, want 1: %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}

	obj := snap.DeprecatedAPIUsage[0]
	if obj.Kind != "Deployment" || obj.Group != "extensions" {
		t.Errorf("DeprecatedAPIUsage[0] = %+v, want extensions/v1beta1 Deployment", obj)
	}
	if obj.Name != "legacy-app-legacy-app" {
		t.Errorf("Name = %q, want legacy-app-legacy-app (release name prefix from the template)", obj.Name)
	}
	wantSource := "helm:" + filepath.Join(repo, "chart")
	if obj.SourcePath != wantSource {
		t.Errorf("SourcePath = %q, want %q", obj.SourcePath, wantSource)
	}
}

func TestCollector_ScanDirAndHelmChart_Combined(t *testing.T) {
	repo := fixtureRepoDir(t)
	c := manifest.NewCollector(
		[]string{filepath.Join(repo, "raw")},
		[]manifest.HelmChart{{Path: filepath.Join(repo, "chart"), ReleaseName: "legacy-app"}},
	)

	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}
	if len(snap.DeprecatedAPIUsage) != 2 {
		t.Fatalf("got %d deprecated API matches combined, want 2 (one raw, one from the chart): %+v", len(snap.DeprecatedAPIUsage), snap.DeprecatedAPIUsage)
	}
}

func TestCollector_NonexistentDirRecordsError(t *testing.T) {
	c := manifest.NewCollector([]string{"/nonexistent/path/does-not-exist"}, nil)
	snap, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect must not return a hard error, only record it: %v", err)
	}
	if len(snap.Errors) != 1 {
		t.Fatalf("expected exactly one recorded error, got %d: %v", len(snap.Errors), snap.Errors)
	}
	got := snap.Errors["manifest-dir:/nonexistent/path/does-not-exist"]
	if got == nil {
		t.Fatalf("missing manifest-dir error: %v", snap.Errors)
	}
	for _, want := range []string{
		"Manifest path not found",
		"Check the path or remove --manifests",
		"/nonexistent/path/does-not-exist",
	} {
		if !strings.Contains(got.Error(), want) {
			t.Errorf("error = %q, want to contain %q", got.Error(), want)
		}
	}
	if strings.Contains(got.Error(), "lstat") {
		t.Errorf("error = %q, should hide raw lstat detail", got.Error())
	}
}

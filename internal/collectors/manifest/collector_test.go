package manifest_test

import (
	"context"
	"path/filepath"
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
}

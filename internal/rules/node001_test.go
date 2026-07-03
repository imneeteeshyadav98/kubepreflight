package rules

import (
	"path/filepath"
	"testing"

	"kubepreflight/internal/findings"
	"kubepreflight/internal/testutil"
)

func TestNODE001_Positive_SkewExceedsPolicy(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "node001", "positive")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (NODE001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "NODE-001" {
		t.Errorf("RuleID = %q, want NODE-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Resource.Name != "ip-10-0-1-11" {
		t.Errorf("Resource.Name = %q, want ip-10-0-1-11", f.Resource.Name)
	}
}

func TestNODE001_Negative_SkewWithinPolicyNoFinding(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "node001", "negative")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (NODE001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (skew within n-3 must not fire): %+v", len(fs), fs)
	}
}

func TestNODE001_NewerKubeletFlagged(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "fixtures", "checks", "node001", "newer")
	objs, err := testutil.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("loading fixtures: %v", err)
	}
	snap := testutil.BuildSnapshot(objs)

	fs, err := (NODE001{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1 (kubelet newer than target must fire): %+v", len(fs), fs)
	}
	if fs[0].Resource.Name != "ip-10-0-1-33" {
		t.Errorf("Resource.Name = %q, want ip-10-0-1-33", fs[0].Resource.Name)
	}
}

func TestParseMajorMinor(t *testing.T) {
	cases := []struct {
		in        string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{"v1.33.0-eks-1234567", 1, 33, false},
		{"1.34", 1, 34, false},
		{"v1.29.3", 1, 29, false},
		{"garbage", 0, 0, true},
	}
	for _, c := range cases {
		major, minor, err := parseMajorMinor(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseMajorMinor(%q): expected error, got none", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseMajorMinor(%q): unexpected error: %v", c.in, err)
			continue
		}
		if major != c.wantMajor || minor != c.wantMinor {
			t.Errorf("parseMajorMinor(%q) = (%d, %d), want (%d, %d)", c.in, major, minor, c.wantMajor, c.wantMinor)
		}
	}
}

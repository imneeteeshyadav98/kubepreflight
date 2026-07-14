package compatcatalog

import (
	"encoding/json"
	"testing"
)

func TestDefaultCatalogValidatesAndIncludesInitialAddons(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	want := map[string]string{
		"eks/vpc-cni":               "v1.18.0-eksbuild.1",
		"eks/kube-proxy":            "v1.34.0-eksbuild.1",
		"eks/coredns":               "v1.11.4-eksbuild.2",
		"eks/aws-ebs-csi-driver":    "v1.44.0-eksbuild.1",
		"eks/aws-efs-csi-driver":    "v2.1.8-eksbuild.1",
		"kubernetes/metrics-server": "v0.7.2",
	}
	for key, min := range want {
		provider, addon := splitCatalogKey(t, key)
		entry, ok := c.Lookup(provider, addon, "v1.34.9")
		if !ok {
			t.Fatalf("Lookup(%s, %s, 1.34) not found", provider, addon)
		}
		if entry.MinimumCompatibleVersion != min {
			t.Fatalf("%s minimum version = %q, want %q", key, entry.MinimumCompatibleVersion, min)
		}
		if entry.Source == "" || entry.Reference == "" || entry.LastVerifiedDate == "" || entry.Confidence == "" {
			t.Fatalf("%s missing source/reference/date/confidence: %+v", key, entry)
		}
	}
}

func TestLookupNormalizesInputAndUnknownStaysUnknown(t *testing.T) {
	c := mustCatalog(t, []Entry{baseEntry()})
	if _, ok := c.Lookup(" EKS ", " VPC-CNI ", "v1.34.3"); !ok {
		t.Fatal("Lookup did not normalize provider/addon/kubernetes version")
	}
	if _, ok := c.Lookup("eks", "vpc-cni", "1.35"); ok {
		t.Fatal("Lookup found missing target version, want unknown")
	}
	if _, ok := c.Lookup("eks", "unknown-addon", "1.34"); ok {
		t.Fatal("Lookup found missing add-on, want unknown")
	}
}

func TestEntriesAreDeterministicallySortedCopies(t *testing.T) {
	c := mustCatalog(t, []Entry{
		{KubernetesVersion: "1.34", Provider: "eks", AddonName: "z-addon", MinimumCompatibleVersion: "v1.0.0", RecommendedVersion: "v1.0.0", Source: "source", Reference: "https://example.com/z", LastVerifiedDate: "2026-07-14", Confidence: "PROVIDER_REPORTED"},
		baseEntry(),
	})
	entries := c.Entries()
	if entries[0].AddonName != "vpc-cni" || entries[1].AddonName != "z-addon" {
		t.Fatalf("Entries order = %+v, want deterministic provider/addon/version order", entries)
	}
	entries[0].AddonName = "mutated"
	again := c.Entries()
	if again[0].AddonName != "vpc-cni" {
		t.Fatal("Entries returned internal slice instead of a copy")
	}
}

func TestValidationRejectsMalformedEntries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Entry)
	}{
		{name: "bad kubernetes version", mutate: func(e *Entry) { e.KubernetesVersion = "1" }},
		{name: "missing provider", mutate: func(e *Entry) { e.Provider = "" }},
		{name: "missing add-on", mutate: func(e *Entry) { e.AddonName = "" }},
		{name: "bad minimum", mutate: func(e *Entry) { e.MinimumCompatibleVersion = "latest" }},
		{name: "bad recommended", mutate: func(e *Entry) { e.RecommendedVersion = "" }},
		{name: "minimum above recommendation", mutate: func(e *Entry) { e.MinimumCompatibleVersion, e.RecommendedVersion = "v2.0.0", "v1.0.0" }},
		{name: "missing source", mutate: func(e *Entry) { e.Source = "" }},
		{name: "missing reference", mutate: func(e *Entry) { e.Reference = "" }},
		{name: "bad verified date", mutate: func(e *Entry) { e.LastVerifiedDate = "20260714" }},
		{name: "bad confidence", mutate: func(e *Entry) { e.Confidence = "MAYBE" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := baseEntry()
			tc.mutate(&entry)
			if _, err := New(Document{SchemaVersion: SchemaVersion, Entries: []Entry{entry}}); err == nil {
				t.Fatal("New succeeded, want validation error")
			}
		})
	}
}

func TestValidationRejectsDuplicateOverlappingEntries(t *testing.T) {
	a, b := baseEntry(), baseEntry()
	b.Provider = " EKS "
	b.AddonName = "VPC-CNI"
	b.KubernetesVersion = "v1.34.9"
	if _, err := New(Document{SchemaVersion: SchemaVersion, Entries: []Entry{a, b}}); err == nil {
		t.Fatal("New succeeded with duplicate normalized key, want overlap error")
	}
}

func TestLoadJSONRejectsWrongSchemaAndBadJSON(t *testing.T) {
	raw, _ := json.Marshal(Document{SchemaVersion: "other", Entries: []Entry{baseEntry()}})
	if _, err := LoadJSON(raw); err == nil {
		t.Fatal("LoadJSON succeeded with wrong schema")
	}
	if _, err := LoadJSON([]byte("{not json")); err == nil {
		t.Fatal("LoadJSON succeeded with invalid JSON")
	}
}

func TestInstalledStatus(t *testing.T) {
	entry := baseEntry()
	entry.MinimumCompatibleVersion = "v1.18.0-eksbuild.1"
	entry.RecommendedVersion = "v1.18.2-eksbuild.1"
	tests := []struct {
		installed string
		want      Status
	}{
		{installed: "v1.17.9-eksbuild.1", want: StatusIncompatible},
		{installed: "v1.18.0-eksbuild.1", want: StatusUpgradeRecommended},
		{installed: "v1.18.2-eksbuild.1", want: StatusCompatible},
		{installed: "", want: StatusUnknown},
		{installed: "latest", want: StatusUnknown},
		{installed: "example.com/addon@sha256:abcdef", want: StatusUnknown},
	}
	for _, tc := range tests {
		if got := entry.InstalledStatus(tc.installed); got != tc.want {
			t.Fatalf("InstalledStatus(%q) = %q, want %q", tc.installed, got, tc.want)
		}
	}
}

func TestCompareVersionsOrdersNumericTokensNumerically(t *testing.T) {
	if CompareVersions("v1.10.0", "v1.9.9") <= 0 {
		t.Fatal("v1.10.0 must sort after v1.9.9")
	}
	if CompareVersions("v1.18.0-eksbuild.2", "v1.18.0-eksbuild.10") >= 0 {
		t.Fatal("eksbuild.2 must sort before eksbuild.10")
	}
	if CompareVersions("v1.18.0", "v1.18.0") != 0 {
		t.Fatal("identical versions must compare equal")
	}
}

func mustCatalog(t *testing.T, entries []Entry) *Catalog {
	t.Helper()
	c, err := New(Document{SchemaVersion: SchemaVersion, Entries: entries})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func baseEntry() Entry {
	return Entry{
		KubernetesVersion:        "1.34",
		Provider:                 "eks",
		AddonName:                "vpc-cni",
		MinimumCompatibleVersion: "v1.18.0-eksbuild.1",
		RecommendedVersion:       "v1.18.1-eksbuild.1",
		Source:                   "AWS EKS DescribeAddonVersions compatibility metadata",
		Reference:                "https://docs.aws.amazon.com/eks/latest/userguide/managing-add-ons.html",
		LastVerifiedDate:         "2026-07-14",
		Confidence:               "PROVIDER_REPORTED",
	}
}

func splitCatalogKey(t *testing.T, key string) (string, string) {
	t.Helper()
	for i, r := range key {
		if r == '/' {
			return key[:i], key[i+1:]
		}
	}
	t.Fatalf("bad test key %q", key)
	return "", ""
}

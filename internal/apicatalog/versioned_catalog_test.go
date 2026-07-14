package apicatalog

import (
	"encoding/json"
	"testing"
)

func TestDefaultVersionedCatalogValidatesAndIncludesSeedEntries(t *testing.T) {
	c, err := DefaultVersioned()
	if err != nil {
		t.Fatalf("DefaultVersioned: %v", err)
	}
	want := []struct {
		group   string
		version string
		kind    string
		target  string
		removed string
	}{
		{group: "policy", version: "v1beta1", kind: "PodSecurityPolicy", target: "1.25", removed: "1.25"},
		{group: "batch", version: "v1beta1", kind: "CronJob", target: "1.34", removed: "1.25"},
		{group: "autoscaling", version: "v2beta2", kind: "HorizontalPodAutoscaler", target: "v1.34.3", removed: "1.26"},
		{group: "flowcontrol.apiserver.k8s.io", version: "v1beta3", kind: "FlowSchema", target: "1.32", removed: "1.32"},
	}
	for _, tc := range want {
		entry, ok := c.Lookup(tc.group, tc.version, tc.kind, tc.target)
		if !ok {
			t.Fatalf("Lookup(%s, %s, %s, %s) not found", tc.group, tc.version, tc.kind, tc.target)
		}
		if entry.RemovedInVersion != tc.removed {
			t.Fatalf("%s/%s %s removedInVersion = %q, want %q", tc.group, tc.version, tc.kind, entry.RemovedInVersion, tc.removed)
		}
		if entry.Source == "" || entry.Reference == "" || entry.LastVerifiedDate == "" || entry.Confidence == "" {
			t.Fatalf("entry missing source metadata: %+v", entry)
		}
	}
}

func TestVersionedLookupNormalizesInputAndTargetRange(t *testing.T) {
	c := mustVersionedCatalog(t, []VersionedAPI{baseVersionedAPI()})
	if _, ok := c.Lookup(" POLICY ", " V1BETA1 ", "PodSecurityPolicy", "v1.34.9"); !ok {
		t.Fatal("Lookup did not normalize group, version, or target patch version")
	}
	if _, ok := c.Lookup("policy", "v1beta1", "PodSecurityPolicy", "1.24"); ok {
		t.Fatal("Lookup found entry before supported target range")
	}
	if _, ok := c.Lookup("policy", "v1beta1", "PodSecurityPolicy", "1.40"); ok {
		t.Fatal("Lookup found entry after supported target range")
	}
	if _, ok := c.Lookup("policy", "v1", "PodSecurityPolicy", "1.34"); ok {
		t.Fatal("Lookup found unknown API version")
	}
}

func TestVersionedEntriesAreDeterministicallySortedCopies(t *testing.T) {
	z := baseVersionedAPI()
	z.Group = "z.example.com"
	z.Kind = "Zed"
	z.Resource = "zeds"
	c := mustVersionedCatalog(t, []VersionedAPI{z, baseVersionedAPI()})

	entries := c.Entries()
	if len(entries) != 2 || entries[0].Kind != "PodSecurityPolicy" || entries[1].Kind != "Zed" {
		t.Fatalf("Entries order = %+v, want deterministic group/version/kind order", entries)
	}
	entries[0].Kind = "mutated"
	again := c.Entries()
	if again[0].Kind != "PodSecurityPolicy" {
		t.Fatal("Entries returned internal slice instead of a copy")
	}
}

func TestVersionedValidationRejectsMalformedEntries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*VersionedAPI)
	}{
		{name: "bad api version", mutate: func(e *VersionedAPI) { e.Version = "1beta1" }},
		{name: "missing resource", mutate: func(e *VersionedAPI) { e.Resource = "" }},
		{name: "missing kind", mutate: func(e *VersionedAPI) { e.Kind = "" }},
		{name: "bad deprecated version", mutate: func(e *VersionedAPI) { e.DeprecatedInVersion = "1" }},
		{name: "bad removed version", mutate: func(e *VersionedAPI) { e.RemovedInVersion = "next" }},
		{name: "deprecated after removed", mutate: func(e *VersionedAPI) { e.DeprecatedInVersion, e.RemovedInVersion = "1.30", "1.29" }},
		{name: "missing replacement", mutate: func(e *VersionedAPI) { e.ReplacementAPI = "" }},
		{name: "bad range min", mutate: func(e *VersionedAPI) { e.SupportedTargetRange.Min = "1" }},
		{name: "bad range max", mutate: func(e *VersionedAPI) { e.SupportedTargetRange.Max = "latest" }},
		{name: "range inverted", mutate: func(e *VersionedAPI) { e.SupportedTargetRange.Min, e.SupportedTargetRange.Max = "1.35", "1.34" }},
		{name: "missing source", mutate: func(e *VersionedAPI) { e.Source = "" }},
		{name: "missing reference", mutate: func(e *VersionedAPI) { e.Reference = "" }},
		{name: "bad verified date", mutate: func(e *VersionedAPI) { e.LastVerifiedDate = "20260714" }},
		{name: "bad confidence", mutate: func(e *VersionedAPI) { e.Confidence = "MAYBE" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entry := baseVersionedAPI()
			tc.mutate(&entry)
			if _, err := NewVersioned(VersionedDocument{SchemaVersion: VersionedSchemaVersion, Entries: []VersionedAPI{entry}}); err == nil {
				t.Fatal("NewVersioned succeeded, want validation error")
			}
		})
	}
}

func TestVersionedValidationRejectsOverlappingEntries(t *testing.T) {
	first := baseVersionedAPI()
	second := baseVersionedAPI()
	second.SupportedTargetRange.Min = "1.30"
	second.SupportedTargetRange.Max = "1.39"
	if _, err := NewVersioned(VersionedDocument{SchemaVersion: VersionedSchemaVersion, Entries: []VersionedAPI{first, second}}); err == nil {
		t.Fatal("NewVersioned succeeded with overlapping ranges, want validation error")
	}
}

func TestLoadVersionedJSONRejectsWrongSchemaAndBadJSON(t *testing.T) {
	raw, _ := json.Marshal(VersionedDocument{SchemaVersion: "other", Entries: []VersionedAPI{baseVersionedAPI()}})
	if _, err := LoadVersionedJSON(raw); err == nil {
		t.Fatal("LoadVersionedJSON succeeded with wrong schema")
	}
	if _, err := LoadVersionedJSON([]byte("{not json")); err == nil {
		t.Fatal("LoadVersionedJSON succeeded with invalid JSON")
	}
}

func mustVersionedCatalog(t *testing.T, entries []VersionedAPI) *VersionedCatalog {
	t.Helper()
	c, err := NewVersioned(VersionedDocument{SchemaVersion: VersionedSchemaVersion, Entries: entries})
	if err != nil {
		t.Fatalf("NewVersioned: %v", err)
	}
	return c
}

func baseVersionedAPI() VersionedAPI {
	return VersionedAPI{
		Group:                "policy",
		Version:              "v1beta1",
		Resource:             "podsecuritypolicies",
		Kind:                 "PodSecurityPolicy",
		Namespaced:           false,
		DeprecatedInVersion:  "1.21",
		RemovedInVersion:     "1.25",
		ReplacementAPI:       "Pod Security Admission or a policy engine (Kyverno/Gatekeeper)",
		SupportedTargetRange: SupportedTargetRange{Min: "1.25", Max: "1.39"},
		Source:               "Kubernetes Deprecated API Migration Guide",
		Reference:            "https://kubernetes.io/docs/reference/using-api/deprecation-guide/#podsecuritypolicy-v125",
		LastVerifiedDate:     "2026-07-14",
		Confidence:           "STATIC_CERTAIN",
	}
}

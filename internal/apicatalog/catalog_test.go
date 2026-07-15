package apicatalog

import (
	"strings"
	"testing"
)

// TestVersionedCatalogCoversLegacyDeprecatedAPIs is the one-time migration
// regression guard for the PR that moved lifecycle data from the
// hand-authored legacyDeprecatedSnapshot into the versioned catalog and
// made Deprecated a derived view of it (legacyFromVersioned). It checks
// the migrated versioned catalog against the FROZEN pre-migration
// snapshot, not against Deprecated itself — Deprecated is now built from
// the same catalog this test verifies, so comparing it to itself would be
// tautological and could never catch a silently dropped or altered entry
// during the migration (or a future accidental edit).
func TestVersionedCatalogCoversLegacyDeprecatedAPIs(t *testing.T) {
	issues, err := LegacyParityIssues()
	if err != nil {
		t.Fatalf("LegacyParityIssues: %v", err)
	}
	for _, issue := range issues {
		t.Error(issue)
	}
}

// legacyEntry is a small builder for synthetic DeprecatedAPI fixtures used
// by the legacyParityIssuesAgainst failure-mode tests below — real field
// values don't matter for these, only whether they match between the two
// sides being compared.
func legacyEntry(group, version, kind string) DeprecatedAPI {
	return DeprecatedAPI{
		Group: group, Version: version, Kind: kind, Resource: "widgets",
		Namespaced: true, RemovedInVersion: "1.25",
		Replacement: "example.com/v1 Widget", ReplacementAPIVersion: "example.com/v1",
	}
}

func TestLegacyParityIssuesAgainst_CleanMatchIsEmpty(t *testing.T) {
	entry := baseVersionedAPI()
	vc := mustVersionedCatalog(t, []VersionedAPI{entry})
	legacy := []DeprecatedAPI{{
		Group: entry.Group, Version: entry.Version, Kind: entry.Kind, Resource: entry.Resource,
		Namespaced: entry.Namespaced, RemovedInVersion: entry.RemovedInVersion,
		Replacement: entry.ReplacementAPI, ReplacementAPIVersion: entry.ReplacementAPIVersion,
	}}
	if issues := legacyParityIssuesAgainst(vc, legacy); len(issues) != 0 {
		t.Fatalf("legacyParityIssuesAgainst = %v, want no issues for a matching pair", issues)
	}
}

func TestLegacyParityIssuesAgainst_MissingGVKFails(t *testing.T) {
	vc := mustVersionedCatalog(t, []VersionedAPI{baseVersionedAPI()}) // only PodSecurityPolicy
	legacy := []DeprecatedAPI{
		legacyEntry("policy", "v1beta1", "PodSecurityPolicy"),
		legacyEntry("batch", "v1beta1", "CronJob"), // not in the catalog
	}
	issues := legacyParityIssuesAgainst(vc, legacy)
	if !containsAny(issues, "missing batch/v1beta1 CronJob") {
		t.Fatalf("issues = %v, want one about the missing CronJob entry", issues)
	}
}

func TestLegacyParityIssuesAgainst_RemovedVersionMismatchFails(t *testing.T) {
	entry := baseVersionedAPI() // removedInVersion 1.25
	vc := mustVersionedCatalog(t, []VersionedAPI{entry})
	legacy := legacyEntry(entry.Group, entry.Version, entry.Kind)
	legacy.RemovedInVersion = "1.26"
	issues := legacyParityIssuesAgainst(vc, []DeprecatedAPI{legacy})
	if !containsAny(issues, "removedInVersion") {
		t.Fatalf("issues = %v, want a removedInVersion mismatch", issues)
	}
}

func TestLegacyParityIssuesAgainst_ReplacementMismatchFails(t *testing.T) {
	entry := baseVersionedAPI()
	vc := mustVersionedCatalog(t, []VersionedAPI{entry})
	legacy := legacyEntry(entry.Group, entry.Version, entry.Kind)
	legacy.Replacement = "something else entirely"
	issues := legacyParityIssuesAgainst(vc, []DeprecatedAPI{legacy})
	if !containsAny(issues, "replacementAPI") {
		t.Fatalf("issues = %v, want a replacementAPI mismatch", issues)
	}
}

func TestLegacyParityIssuesAgainst_DuplicateLegacyEntryFails(t *testing.T) {
	entry := baseVersionedAPI()
	vc := mustVersionedCatalog(t, []VersionedAPI{entry})
	legacy := legacyEntry(entry.Group, entry.Version, entry.Kind)
	issues := legacyParityIssuesAgainst(vc, []DeprecatedAPI{legacy, legacy})
	if !containsAny(issues, "duplicate lifecycle definition") {
		t.Fatalf("issues = %v, want a duplicate-definition issue", issues)
	}
}

func containsAny(issues []string, substr string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, substr) {
			return true
		}
	}
	return false
}

// PodSecurityPolicy was removed twice, from two different API surfaces,
// with two different kinds of replacement: extensions/v1beta1 (1.16) had a
// direct version-swap replacement (policy/v1beta1), while policy/v1beta1
// itself (1.25) retired PSP entirely with no replacement API version at
// all. The exemption below is keyed on Group+Version, not just Kind, so it
// can't accidentally exempt the wrong one of the two.
func TestReplacementAPIVersion_SetForDirectVersionSwaps(t *testing.T) {
	for _, d := range Deprecated {
		if d.Group == "policy" && d.Version == "v1beta1" && d.Kind == "PodSecurityPolicy" {
			continue
		}
		if d.ReplacementAPIVersion == "" {
			t.Errorf("%s %s/%s: ReplacementAPIVersion is empty, want a direct apiVersion swap value", d.Kind, d.Group, d.Version)
		}
	}
}

func TestReplacementAPIVersion_EmptyForFinalPodSecurityPolicyRemoval(t *testing.T) {
	for _, d := range Deprecated {
		if d.Group != "policy" || d.Version != "v1beta1" || d.Kind != "PodSecurityPolicy" {
			continue
		}
		if d.ReplacementAPIVersion != "" {
			t.Errorf("policy/v1beta1 PodSecurityPolicy: ReplacementAPIVersion = %q, want empty (its 1.25 removal retired PSP entirely — no replacement API version)", d.ReplacementAPIVersion)
		}
		return
	}
	t.Fatal("policy/v1beta1 PodSecurityPolicy entry not found in Deprecated")
}

func TestReplacementAPIVersion_SetForExtensionsPodSecurityPolicy(t *testing.T) {
	for _, d := range Deprecated {
		if d.Group != "extensions" || d.Version != "v1beta1" || d.Kind != "PodSecurityPolicy" {
			continue
		}
		if d.ReplacementAPIVersion != "policy/v1beta1" {
			t.Errorf("extensions/v1beta1 PodSecurityPolicy: ReplacementAPIVersion = %q, want policy/v1beta1 (its 1.16 removal's official migration target)", d.ReplacementAPIVersion)
		}
		return
	}
	t.Fatal("extensions/v1beta1 PodSecurityPolicy entry not found in Deprecated")
}

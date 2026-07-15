package apicatalog

import "testing"

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
	vc, err := DefaultVersioned()
	if err != nil {
		t.Fatalf("DefaultVersioned: %v", err)
	}

	seen := make(map[string]bool, len(legacyDeprecatedSnapshot))
	for _, legacy := range legacyDeprecatedSnapshot {
		key := legacy.Group + "/" + legacy.Version + " " + legacy.Kind
		if seen[key] {
			t.Errorf("legacyDeprecatedSnapshot has a duplicate lifecycle definition for %s", key)
		}
		seen[key] = true

		entry, ok := vc.EntryFor(legacy.Group, legacy.Version, legacy.Kind)
		if !ok {
			t.Errorf("versioned catalog is missing %s — every legacy GVK must be present, none may be silently omitted", key)
			continue
		}
		if entry.RemovedInVersion != legacy.RemovedInVersion {
			t.Errorf("%s: versioned removedInVersion = %q, want %q (must match legacy)", key, entry.RemovedInVersion, legacy.RemovedInVersion)
		}
		if entry.ReplacementAPI != legacy.Replacement {
			t.Errorf("%s: versioned replacementAPI = %q, want %q (must match legacy)", key, entry.ReplacementAPI, legacy.Replacement)
		}
		if entry.ReplacementAPIVersion != legacy.ReplacementAPIVersion {
			t.Errorf("%s: versioned replacementAPIVersion = %q, want %q (must match legacy)", key, entry.ReplacementAPIVersion, legacy.ReplacementAPIVersion)
		}
		if entry.Namespaced != legacy.Namespaced {
			t.Errorf("%s: versioned namespaced = %v, want %v (must match legacy)", key, entry.Namespaced, legacy.Namespaced)
		}
		if entry.Resource != legacy.Resource {
			t.Errorf("%s: versioned resource = %q, want %q (must match legacy)", key, entry.Resource, legacy.Resource)
		}
	}

	if got, want := len(vc.Entries()), len(legacyDeprecatedSnapshot); got != want {
		t.Errorf("versioned catalog has %d entries, want exactly %d (same count as the migrated legacy snapshot — extra entries beyond a 1:1 migration belong in a dedicated maintenance PR, not silently alongside it)", got, want)
	}
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

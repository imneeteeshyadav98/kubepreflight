package apicatalog

import "testing"

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

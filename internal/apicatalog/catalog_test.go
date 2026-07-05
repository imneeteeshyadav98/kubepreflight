package apicatalog

import "testing"

func TestReplacementAPIVersion_SetForDirectVersionSwaps(t *testing.T) {
	for _, d := range Deprecated {
		if d.Kind == "PodSecurityPolicy" {
			continue
		}
		if d.ReplacementAPIVersion == "" {
			t.Errorf("%s: ReplacementAPIVersion is empty, want a direct apiVersion swap value", d.Kind)
		}
	}
}

func TestReplacementAPIVersion_EmptyForPodSecurityPolicy(t *testing.T) {
	for _, d := range Deprecated {
		if d.Kind != "PodSecurityPolicy" {
			continue
		}
		if d.ReplacementAPIVersion != "" {
			t.Errorf("PodSecurityPolicy: ReplacementAPIVersion = %q, want empty (its replacement isn't a version bump)", d.ReplacementAPIVersion)
		}
		return
	}
	t.Fatal("PodSecurityPolicy entry not found in Deprecated")
}

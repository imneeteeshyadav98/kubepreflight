package redact

import "testing"

// Realistic values, not placeholders — a regex that only catches
// "example.com"-style test fixtures wouldn't prove anything about the real
// leak this package exists to close (see demo/live-eks/redact-evidence.sh,
// the stopgap this package replaces).
const (
	realARN      = "arn:aws:eks:us-east-1:164761934067:cluster/kubepreflight-live-demo"
	realHostname = "ip-192-168-1-73.ec2.internal"
)

func TestText_RedactsARN(t *testing.T) {
	in := "cluster context: " + realARN
	out := Text(in)
	if out == in {
		t.Fatal("Text did not change a string containing a real ARN")
	}
	if got := Text(realARN); got != ARNPlaceholder {
		t.Errorf("Text(%q) = %q, want %q", realARN, got, ARNPlaceholder)
	}
}

func TestText_RedactsHostname(t *testing.T) {
	in := "qualifying node(s): " + realHostname
	out := Text(in)
	if out == in {
		t.Fatal("Text did not change a string containing a real node hostname")
	}
	if got := Text(realHostname); got != HostnamePlaceholder {
		t.Errorf("Text(%q) = %q, want %q", realHostname, got, HostnamePlaceholder)
	}
}

func TestText_RedactsHostnameComputeInternalVariant(t *testing.T) {
	host := "ip-10-0-1-100.us-east-1.compute.internal"
	if got := Text(host); got != HostnamePlaceholder {
		t.Errorf("Text(%q) = %q, want %q", host, got, HostnamePlaceholder)
	}
}

func TestText_RedactsBothInOneString(t *testing.T) {
	in := "cluster " + realARN + " node " + realHostname + " is healthy"
	out := Text(in)
	want := "cluster " + ARNPlaceholder + " node " + HostnamePlaceholder + " is healthy"
	if out != want {
		t.Errorf("Text(%q) = %q, want %q", in, out, want)
	}
}

func TestText_NonSensitiveStringsUnchanged(t *testing.T) {
	cases := []string{
		"",
		"critical-app-pdb",
		"PodDisruptionBudget/preflight-lab/critical-app-pdb",
		"3952b89010b14ff47d40c79871c65d44ca212b804899227337ffd396a46be4da", // a fingerprint hash
		"kube-system/coredns",
		"us-east-1",
		"1.35",
	}
	for _, s := range cases {
		if got := Text(s); got != s {
			t.Errorf("Text(%q) = %q, want unchanged", s, got)
		}
	}
}

func TestText_DoesNotOverRedactFingerprintHashes(t *testing.T) {
	// A SHA-256 fingerprint is 64 hex characters — long enough to
	// coincidentally contain 12 consecutive digits, which is exactly the
	// false-positive this package's own account-ID-shaped substrings must
	// not trigger on (the ARN pattern requires the literal "arn:aws:"
	// prefix, so a bare hex string can never match it).
	hash := "6932c5068e72908a551ea7a5888c4ad91c37cd9b8905449387696da3bb784f9f"
	if got := Text(hash); got != hash {
		t.Errorf("Text(%q) = %q, want unchanged (fingerprint hash misidentified as sensitive)", hash, got)
	}
}

func TestStrings_PreservesNilVsEmpty(t *testing.T) {
	if got := Strings(nil); got != nil {
		t.Errorf("Strings(nil) = %#v, want nil", got)
	}
	got := Strings([]string{})
	if got == nil || len(got) != 0 {
		t.Errorf("Strings([]string{}) = %#v, want non-nil empty slice", got)
	}
}

func TestStrings_RedactsEveryElement(t *testing.T) {
	in := []string{"qualifying node(s): " + realHostname, "unrelated-value"}
	out := Strings(in)
	if out[0] != "qualifying node(s): "+HostnamePlaceholder {
		t.Errorf("Strings()[0] = %q, want redacted", out[0])
	}
	if out[1] != "unrelated-value" {
		t.Errorf("Strings()[1] = %q, want unchanged", out[1])
	}
}

func TestStringMapValues_RedactsValuesNotKeys(t *testing.T) {
	m := map[string]string{"resourceArn": realARN}
	StringMapValues(m)
	if m["resourceArn"] != ARNPlaceholder {
		t.Errorf("StringMapValues did not redact the value: %q", m["resourceArn"])
	}
}

package redact

import "testing"

// Realistic values, not placeholders — a regex that only catches
// "example.com"-style test fixtures wouldn't prove anything about the real
// leak this package exists to close (see demo/live-eks/redact-evidence.sh,
// the stopgap this package replaces).
const (
	realARN       = "arn:aws:eks:us-east-1:164761934067:cluster/kubepreflight-live-demo"
	realHostname  = "ip-192-168-1-73.ec2.internal"
	realAccountID = "164761934067"
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

// TestText_RedactsStandaloneAccountID guards SEC-TRUST-001: a bare
// 12-digit account ID in free text (no "arn:aws:" prefix) is a real leak
// path arnPattern alone cannot catch, e.g. an AWS error message that
// names the account without embedding a full ARN.
func TestText_RedactsStandaloneAccountID(t *testing.T) {
	in := "AccessDenied for account " + realAccountID
	out := Text(in)
	want := "AccessDenied for account " + AccountIDPlaceholder
	if out != want {
		t.Errorf("Text(%q) = %q, want %q", in, out, want)
	}
	if got := Text(realAccountID); got != AccountIDPlaceholder {
		t.Errorf("Text(%q) = %q, want %q", realAccountID, got, AccountIDPlaceholder)
	}
}

// TestText_ARNAccountIDNotDoubleRedacted guards that the account ID
// pattern doesn't run against what's left of an ARN after arnPattern
// already replaced it — the whole ARN (account ID included) becomes one
// ARNPlaceholder, never "[redacted-arn]" plus a leftover/second
// account-ID placeholder.
func TestText_ARNAccountIDNotDoubleRedacted(t *testing.T) {
	out := Text(realARN)
	if out != ARNPlaceholder {
		t.Errorf("Text(%q) = %q, want exactly %q (not double-redacted)", realARN, out, ARNPlaceholder)
	}
}

// TestText_RedactsAllThreePatternsInOneString guards the three sensitive
// patterns this package now covers (ARN, standalone account ID, EC2
// internal hostname) all fire independently within a single string, the
// realistic shape of a coverage error or evidence line that mentions more
// than one identifier.
func TestText_RedactsAllThreePatternsInOneString(t *testing.T) {
	in := "cluster " + realARN + " account " + realAccountID + " node " + realHostname
	out := Text(in)
	want := "cluster " + ARNPlaceholder + " account " + AccountIDPlaceholder + " node " + HostnamePlaceholder
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
	// coincidentally contain runs of consecutive digits, which is exactly
	// the false-positive the account-ID pattern must not trigger on: a
	// digit run adjacent to a hex letter has no \b boundary there (letters
	// are word characters too), so \b\d{12}\b never fires inside it. The
	// ARN pattern separately can't match since it requires the literal
	// "arn:aws:" prefix, which a bare hex string never has.
	hash := "6932c5068e72908a551ea7a5888c4ad91c37cd9b8905449387696da3bb784f9f"
	if got := Text(hash); got != hash {
		t.Errorf("Text(%q) = %q, want unchanged (fingerprint hash misidentified as sensitive)", hash, got)
	}
}

// TestText_DoesNotOverRedactNonAccountIDDigitRuns guards the account-ID
// pattern's exact-12-digit boundary: neither a shorter nor a longer bare
// digit run (a resource count, a Unix-ms timestamp) is mistaken for an
// account ID. Only an exact 12-digit run, bounded by non-digit characters
// on both sides, qualifies.
func TestText_DoesNotOverRedactNonAccountIDDigitRuns(t *testing.T) {
	cases := []string{
		"12345678901",   // 11 digits — one short
		"1234567890123", // 13 digits — one over
		"1700000000000", // 13-digit Unix-ms timestamp
	}
	for _, s := range cases {
		if got := Text(s); got != s {
			t.Errorf("Text(%q) = %q, want unchanged (not a 12-digit account ID)", s, got)
		}
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

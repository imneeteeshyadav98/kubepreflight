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

func TestText_RedactsAWSInfrastructureIdentifiers(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"vpc", "vpc-0123456789abcdef0", VPCIDPlaceholder},
		{"subnet", "subnet-0123456789abcdef0", SubnetIDPlaceholder},
		{"security group", "sg-0123456789abcdef0", SecurityGroupIDPlaceholder},
		{"instance", "i-0123456789abcdef0", InstanceIDPlaceholder},
		{"volume", "vol-0123456789abcdef0", VolumeIDPlaceholder},
		{"eni", "eni-0123456789abcdef0", NetworkInterfaceIDPlaceholder},
		{"route table", "rtb-0123456789abcdef0", RouteTableIDPlaceholder},
		{"internet gateway", "igw-0123456789abcdef0", InternetGatewayIDPlaceholder},
		{"nat gateway", "nat-0123456789abcdef0", NATGatewayIDPlaceholder},
		{"eip allocation", "eipalloc-0123456789abcdef0", EIPAllocationIDPlaceholder},
		{"launch template", "lt-0123456789abcdef0", LaunchTemplateIDPlaceholder},
		{"punctuation", "subnets: [subnet-0123456789abcdef0], sg=sg-0123456789abcdef0.", "subnets: [" + SubnetIDPlaceholder + "], sg=" + SecurityGroupIDPlaceholder + "."},
		{"json string", `"VpcId":"vpc-0123456789abcdef0"`, `"VpcId":"` + VPCIDPlaceholder + `"`},
		{"markdown", "`vol-0123456789abcdef0`", "`" + VolumeIDPlaceholder + "`"},
		{"multiple", "vpc-0123456789abcdef0 subnet-0123456789abcdef0 i-0123456789abcdef0", VPCIDPlaceholder + " " + SubnetIDPlaceholder + " " + InstanceIDPlaceholder},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.in); got != tt.want {
				t.Errorf("Text(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestText_RedactsEndpointIPsPathsAndTokens(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"eks url", "server https://ABCDEF.gr7.us-east-1.eks." + "amazonaws.com", "server " + EKSURLPlaceholder},
		{"private ip", "node 10.0.12.34 ready", "node " + IPPlaceholder + " ready"},
		{"public ip", "egress 203.0.113.10", "egress " + IPPlaceholder},
		{"unix path", "kubeconfig=/home/alice/.kube/config", "kubeconfig=" + PathPlaceholder},
		{"windows path", `kubeconfig=C:\Users\alice\.kube\config`, "kubeconfig=" + PathPlaceholder},
		{"access key", "key AKIA" + "0123456789ABCDEF", "key " + AccessKeyPlaceholder},
		{"session access key", "key ASIA" + "0123456789ABCDEF", "key " + AccessKeyPlaceholder},
		{"bearer token", "Authorization: Bearer " + "abc.def_ghi-123", "Authorization: " + TokenPlaceholder},
		{"session token", "aws_" + "session_" + "token = abc/def+ghi=", AWSSessionTokenPlaceholder},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.in); got != tt.want {
				t.Errorf("Text(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestText_DoesNotOverRedactAWSIdentifierNearMisses(t *testing.T) {
	cases := []string{
		"i-runner",
		"i-am-a-message",
		"i-abc",
		"i-0123456g",
		"sg-test",
		"volume-id",
		"subnetting",
		"vpc-main",
		"vpc-network",
		"RED-CLOUD-ID-002",
		"API-001",
		"1.35",
		"v1.35.0",
		"already " + VPCIDPlaceholder,
		"6932c5068e72908a551ea7a5888c4ad91c37cd9b8905449387696da3bb784f9f",
	}
	for _, s := range cases {
		if got := Text(s); got != s {
			t.Errorf("Text(%q) = %q, want unchanged", s, got)
		}
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

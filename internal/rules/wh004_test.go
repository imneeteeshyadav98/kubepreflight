package rules

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// wh004CertPEM generates a real, self-signed X.509 certificate PEM block
// with the given validity window and CA basic constraint, so these tests
// exercise the actual crypto/x509 parsing path rather than hand-built byte
// fixtures.
func wh004CertPEM(t *testing.T, notBefore, notAfter time.Time, isCA bool) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "wh004-test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		// BasicConstraintsValid must be true for IsCA to actually be
		// encoded into the certificate's Basic Constraints extension --
		// without it, x509.CreateCertificate silently ignores IsCA and
		// every parsed-back certificate reports IsCA()==false regardless
		// of what's set here.
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func wh004HealthyCABundle(t *testing.T) []byte {
	t.Helper()
	now := time.Now()
	return wh004CertPEM(t, now.Add(-24*time.Hour), now.Add(365*24*time.Hour), true)
}

func wh004WebhookConfig(webhooks ...admissionregistrationv1.ValidatingWebhook) admissionregistrationv1.ValidatingWebhookConfiguration {
	return admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "guard", UID: "uid-guard"},
		Webhooks:   webhooks,
	}
}

func wh004RequireOne(t *testing.T, fs []findings.Finding) findings.Finding {
	t.Helper()
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].RuleID != "WH-004" {
		t.Errorf("RuleID = %q, want WH-004", fs[0].RuleID)
	}
	return fs[0]
}

func TestWH004_HealthyConfigIsClean(t *testing.T) {
	fail := admissionregistrationv1.Fail
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			CABundle: wh004HealthyCABundle(t),
		})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a healthy config", fs, err)
	}
}

func TestWH004_NoClientConfigDeferredToWH002(t *testing.T) {
	fail := admissionregistrationv1.Fail
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no WH-004 findings -- WH-002 owns this case", fs, err)
	}
}

func TestWH004_EmptyCABundle_AlwaysWarningRegardlessOfFailurePolicy(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	for name, fp := range map[string]*admissionregistrationv1.FailurePolicyType{"Fail": &fail, "Ignore": &ignore} {
		t.Run(name, func(t *testing.T) {
			snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
				wh004WebhookConfig(wh002Webhook("guard.example.com", fp, admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
				})),
			}}
			fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			f := wh004RequireOne(t, fs)
			if f.Severity != findings.SeverityWarning {
				t.Errorf("Severity = %q, want Warning", f.Severity)
			}
			if f.GlobalBlocker {
				t.Error("GlobalBlocker = true, want false -- an empty caBundle isn't unconditionally broken")
			}
		})
	}
}

func TestWH004_InvalidPEM(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	t.Run("Fail is a Blocker", func(t *testing.T) {
		snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
			wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
				Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
				CABundle: []byte("this is not PEM data at all"),
			})),
		}}
		fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh004RequireOne(t, fs)
		if f.Severity != findings.SeverityBlocker {
			t.Errorf("Severity = %q, want Blocker", f.Severity)
		}
	})

	t.Run("Ignore is a Warning", func(t *testing.T) {
		snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
			wh004WebhookConfig(wh002Webhook("guard.example.com", &ignore, admissionregistrationv1.WebhookClientConfig{
				Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
				CABundle: []byte("this is not PEM data at all"),
			})),
		}}
		fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh004RequireOne(t, fs)
		if f.Severity != findings.SeverityWarning {
			t.Errorf("Severity = %q, want Warning", f.Severity)
		}
	})
}

func TestWH004_PEMWithNoCertificateBlock(t *testing.T) {
	fail := admissionregistrationv1.Fail
	// A real PEM structure, but not a CERTIFICATE block -- e.g. someone
	// pasted a private key into caBundle by mistake.
	notACert := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("fake-key-bytes")})
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			CABundle: notACert,
		})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh004RequireOne(t, fs)
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if !strings.Contains(f.Message, "no CERTIFICATE blocks") {
		t.Errorf("Message = %q, want it to mention no CERTIFICATE blocks", f.Message)
	}
}

func TestWH004_ExpiredCertificate(t *testing.T) {
	fail := admissionregistrationv1.Fail
	now := time.Now()
	expired := wh004CertPEM(t, now.Add(-2*365*24*time.Hour), now.Add(-24*time.Hour), true)
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			CABundle: expired,
		})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh004RequireOne(t, fs)
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if !strings.Contains(f.Message, "expired") {
		t.Errorf("Message = %q, want it to mention the certificate expired", f.Message)
	}
}

func TestWH004_ExpiredCertificate_GlobalBlockerWhenCatchAllScope(t *testing.T) {
	fail := admissionregistrationv1.Fail
	now := time.Now()
	expired := wh004CertPEM(t, now.Add(-2*365*24*time.Hour), now.Add(-24*time.Hour), true)
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{{
		ObjectMeta: metav1.ObjectMeta{Name: "catch-all-guard", UID: "uid-catch-all"},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name:          "catchall.example.com",
			FailurePolicy: &fail,
			Rules: []admissionregistrationv1.RuleWithOperations{
				{Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll}, Rule: admissionregistrationv1.Rule{APIGroups: []string{"*"}, Resources: []string{"*"}}},
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
				CABundle: expired,
			},
		}},
	}}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh004RequireOne(t, fs)
	if !f.GlobalBlocker {
		t.Error("GlobalBlocker = false, want true (catch-all scope + fail-closed + expired CA cert)")
	}
}

func TestWH004_CertificateExpiringSoon(t *testing.T) {
	fail := admissionregistrationv1.Fail
	now := time.Now()
	expiringSoon := wh004CertPEM(t, now.Add(-24*time.Hour), now.Add(10*24*time.Hour), true)
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			CABundle: expiringSoon,
		})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh004RequireOne(t, fs)
	// Warning even with failurePolicy: Fail -- it hasn't expired yet.
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning (not yet expired)", f.Severity)
	}
	if f.GlobalBlocker {
		t.Error("GlobalBlocker = true, want false -- not currently broken")
	}
}

func TestWH004_CertificateNotMarkedAsCA(t *testing.T) {
	fail := admissionregistrationv1.Fail
	now := time.Now()
	notCA := wh004CertPEM(t, now.Add(-24*time.Hour), now.Add(365*24*time.Hour), false)
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			CABundle: notCA,
		})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	f := wh004RequireOne(t, fs)
	if f.Severity != findings.SeverityWarning {
		t.Errorf("Severity = %q, want Warning", f.Severity)
	}
}

func TestWH004_MultipleSimultaneousCertProblems(t *testing.T) {
	fail := admissionregistrationv1.Fail
	now := time.Now()
	// Expired AND not marked as CA -- two distinct problems on one cert.
	badCert := wh004CertPEM(t, now.Add(-2*365*24*time.Hour), now.Add(-24*time.Hour), false)
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{
			Service:  &admissionregistrationv1.ServiceReference{Namespace: "guard-ns", Name: "guard-svc"},
			CABundle: badCert,
		})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2 (expired + not-CA): %+v", len(fs), fs)
	}
	if fs[0].Fingerprint == fs[1].Fingerprint {
		t.Errorf("both findings share fingerprint %q, want distinct", fs[0].Fingerprint)
	}
}

func TestWH004_InsecureURL(t *testing.T) {
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore

	t.Run("Fail is a Blocker", func(t *testing.T) {
		url := "http://webhook.example.com/validate"
		snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
			// CABundle set to a healthy cert so only the URL-scheme check
			// is under test here -- caBundle applies to URL-based
			// clientConfig too (it's not nested under ServiceReference),
			// and an unset one would also (correctly) fire its own
			// separate empty-caBundle finding.
			wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{URL: &url, CABundle: wh004HealthyCABundle(t)})),
		}}
		fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh004RequireOne(t, fs)
		if f.Severity != findings.SeverityBlocker {
			t.Errorf("Severity = %q, want Blocker", f.Severity)
		}
	})

	t.Run("Ignore is a Warning", func(t *testing.T) {
		url := "http://webhook.example.com/validate"
		snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
			wh004WebhookConfig(wh002Webhook("guard.example.com", &ignore, admissionregistrationv1.WebhookClientConfig{URL: &url, CABundle: wh004HealthyCABundle(t)})),
		}}
		fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		f := wh004RequireOne(t, fs)
		if f.Severity != findings.SeverityWarning {
			t.Errorf("Severity = %q, want Warning", f.Severity)
		}
	})
}

func TestWH004_HTTPSURLIsClean(t *testing.T) {
	fail := admissionregistrationv1.Fail
	url := "https://webhook.example.com/validate"
	snap := &k8s.Snapshot{Errors: map[string]error{}, ValidatingWebhookConfigs: []admissionregistrationv1.ValidatingWebhookConfiguration{
		wh004WebhookConfig(wh002Webhook("guard.example.com", &fail, admissionregistrationv1.WebhookClientConfig{URL: &url, CABundle: wh004HealthyCABundle(t)})),
	}}
	fs, err := (WH004{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a valid https:// URL with a healthy caBundle", fs, err)
	}
}

package rules

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/findings"
)

// wh004CertExpiryWarningWindow: a CA certificate expiring within this
// window is flagged as a Warning even though it's still technically valid
// today — an admission webhook silently going dark on expiry is exactly
// the kind of failure worth surfacing before it happens, not after.
const wh004CertExpiryWarningWindow = 30 * 24 * time.Hour

// WH004 flags webhook TLS/CA configuration that's deterministically broken
// from the object's own data alone: an invalid https:// URL scheme, or a
// caBundle that's empty, unparseable, expired, expiring soon, or contains
// a certificate not marked as a CA. This tool never makes a network call
// to the webhook itself, so it deliberately stops at what's provable from
// the stored bytes — hostname verification and full chain-of-trust
// validation would require actually connecting, which is out of scope.
//
// Severity follows failurePolicy the same way WH-002 does: a broken CA
// bundle behaves exactly like an unreachable backend from the API
// server's perspective (the TLS handshake fails, so the webhook can't be
// called), so Fail-closed makes it a Blocker and Ignore makes it a
// Warning. The three forward-looking/soft signals (empty caBundle,
// expiring-soon certificate, non-CA certificate) stay Warning regardless
// of failurePolicy -- none of them is unconditionally broken today.
type WH004 struct{}

func (WH004) ID() string { return "WH-004" }

func (WH004) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	if snap == nil {
		return nil, nil
	}
	var out []findings.Finding
	now := time.Now()

	for _, cfg := range snap.ValidatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			out = append(out, wh004EvaluateWebhook(wh004Input{
				Kind:       "ValidatingWebhookConfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicy: wh.FailurePolicy, ClientConfig: wh.ClientConfig,
				Rules: wh.Rules, NamespaceSelector: wh.NamespaceSelector, ObjectSelector: wh.ObjectSelector,
				HasMatchConditions: len(wh.MatchConditions) > 0,
			}, now, targetVersion)...)
		}
	}
	for _, cfg := range snap.MutatingWebhookConfigs {
		for i, wh := range cfg.Webhooks {
			out = append(out, wh004EvaluateWebhook(wh004Input{
				Kind:       "MutatingWebhookConfiguration",
				ConfigName: cfg.Name, ConfigUID: string(cfg.UID),
				WebhookName: wh.Name, WebhookIndex: i,
				FailurePolicy: wh.FailurePolicy, ClientConfig: wh.ClientConfig,
				Rules: wh.Rules, NamespaceSelector: wh.NamespaceSelector, ObjectSelector: wh.ObjectSelector,
				HasMatchConditions: len(wh.MatchConditions) > 0,
			}, now, targetVersion)...)
		}
	}
	return out, nil
}

type wh004Input struct {
	Kind                  string
	ConfigName, ConfigUID string
	WebhookName           string
	WebhookIndex          int
	FailurePolicy         *admissionregistrationv1.FailurePolicyType
	ClientConfig          admissionregistrationv1.WebhookClientConfig
	Rules                 []admissionregistrationv1.RuleWithOperations
	NamespaceSelector     *metav1.LabelSelector
	ObjectSelector        *metav1.LabelSelector
	HasMatchConditions    bool
}

func wh004EvaluateWebhook(in wh004Input, now time.Time, targetVersion string) []findings.Finding {
	if in.ClientConfig.Service == nil && in.ClientConfig.URL == nil {
		// WH-002 already flags a clientConfig with neither service nor
		// url set; nothing TLS-specific to check without one.
		return nil
	}
	ref := findings.LiveResource(in.Kind, findings.ScopeCluster, "", in.ConfigName, in.ConfigUID)
	failClosed := isFailClosed(in.FailurePolicy)
	// Same reasoning as WH-002: a catch-all-scope Fail-closed webhook the
	// API server can't establish TLS with is just as much a global
	// blocker as one that's simply unreachable. Only applied to the
	// definitely-broken-now cases below (insecure URL, unparseable/
	// expired caBundle) -- a certificate that's merely expiring soon or
	// not CA-flagged isn't currently blocking anything.
	global := failClosed && hasGlobalWriteScope(in.Rules, in.NamespaceSelector, in.ObjectSelector, in.HasMatchConditions)
	var out []findings.Finding

	if in.ClientConfig.URL != nil && !strings.HasPrefix(*in.ClientConfig.URL, "https://") {
		out = append(out, wh004Finding(in, ref, wh004Severity(failClosed), global, "insecure-url", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has a url that doesn't start with https:// — the API server requires https and will refuse to call it", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), fmt.Sprintf("url: %s", *in.ClientConfig.URL)},
			"Update the webhook's clientConfig.url to use the https:// scheme.",
		))
	}

	out = append(out, wh004CABundleFindings(in, ref, failClosed, global, now, targetVersion)...)

	return out
}

func wh004Severity(failClosed bool) findings.Severity {
	if failClosed {
		return findings.SeverityBlocker
	}
	return findings.SeverityWarning
}

// wh004CABundleFindings checks caBundle in isolation from Service/URL
// reachability (WH-002's concern) -- a perfectly healthy backend is still
// unreachable if the API server can't validate its TLS certificate.
func wh004CABundleFindings(in wh004Input, ref findings.ResourceReference, failClosed, global bool, now time.Time, targetVersion string) []findings.Finding {
	if len(in.ClientConfig.CABundle) == 0 {
		// "If unspecified, system trust roots on the apiserver are used"
		// (admissionregistration/v1's own doc comment) -- a genuinely
		// valid choice for a webhook whose certificate is already
		// system-trusted, so this is a Warning to double-check, never a
		// Blocker, and never failurePolicy-dependent -- it isn't
		// unconditionally broken, so never a GlobalBlocker either.
		return []findings.Finding{wh004Finding(in, ref, findings.SeverityWarning, false, "empty-ca-bundle", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has no caBundle set — the API server falls back to its system trust roots, which won't validate a self-signed or cluster-internal certificate", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "caBundle: not set"},
			"If the webhook backend uses a self-signed or cluster-internal certificate (the common case for in-cluster webhooks), set clientConfig.caBundle to the CA that signed it. If the backend's certificate is already signed by a system-trusted CA, no action is needed.",
		)}
	}

	certs, unparseable := wh004ParseCABundle(in.ClientConfig.CABundle)
	if unparseable {
		return []findings.Finding{wh004Finding(in, ref, wh004Severity(failClosed), global, "invalid-ca-bundle", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has a caBundle that isn't valid PEM-encoded certificate data — the API server can't use it to validate the webhook's TLS certificate", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "caBundle: not valid PEM-encoded certificate data"},
			"Replace clientConfig.caBundle with a valid PEM-encoded CA certificate (or chain).",
		)}
	}
	if len(certs) == 0 {
		// PEM-decodable but no CERTIFICATE blocks at all (e.g. only a
		// private key was pasted in by mistake) -- equally unusable.
		return []findings.Finding{wh004Finding(in, ref, wh004Severity(failClosed), global, "invalid-ca-bundle", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has a caBundle with no CERTIFICATE blocks — the API server can't use it to validate the webhook's TLS certificate", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "caBundle: no CERTIFICATE blocks found"},
			"Replace clientConfig.caBundle with a valid PEM-encoded CA certificate (or chain).",
		)}
	}

	var out []findings.Finding
	expired := false
	expiringSoon := false
	notCA := false
	for _, cert := range certs {
		if now.After(cert.NotAfter) {
			expired = true
		} else if cert.NotAfter.Sub(now) <= wh004CertExpiryWarningWindow {
			expiringSoon = true
		}
		if !cert.IsCA {
			notCA = true
		}
	}

	if expired {
		out = append(out, wh004Finding(in, ref, wh004Severity(failClosed), global, "expired-ca-cert", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has a caBundle containing an expired certificate — the API server can no longer validate the webhook's TLS certificate against it", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "caBundle: contains an expired certificate"},
			"Rotate the CA certificate referenced by clientConfig.caBundle (and the webhook's own serving certificate if it was signed by the same CA).",
		))
	} else if expiringSoon {
		out = append(out, wh004Finding(in, ref, findings.SeverityWarning, false, "ca-cert-expiring-soon", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has a caBundle containing a certificate expiring within %d days", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex, int(wh004CertExpiryWarningWindow.Hours()/24)),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "caBundle: contains a certificate expiring soon"},
			"Plan a CA certificate rotation before it expires — an expired caBundle will make the API server unable to call this webhook at all.",
		))
	}

	if notCA {
		out = append(out, wh004Finding(in, ref, findings.SeverityWarning, false, "ca-cert-not-marked-ca", targetVersion,
			fmt.Sprintf("%s %q: webhook %q (index %d in .webhooks) has a caBundle containing a certificate without the CA basic constraint set — this only works if it's the exact leaf certificate being pinned, not a genuine CA", in.Kind, in.ConfigName, in.WebhookName, in.WebhookIndex),
			[]string{fmt.Sprintf("webhook name: %s", in.WebhookName), "caBundle: contains a certificate not marked as a CA"},
			"Confirm this is intentional certificate pinning (the exact serving certificate, not a CA). If a CA certificate was intended, verify the correct one was used.",
		))
	}

	return out
}

// wh004ParseCABundle decodes every PEM block in raw and parses the
// CERTIFICATE ones as X.509 certificates. unparseable is true only when
// raw contains no PEM structure at all -- a bundle that parses as PEM but
// fails to parse as a certificate, or contains no CERTIFICATE blocks, is
// reported by the caller as an empty certs slice instead, since those are
// distinguishable failure modes worth different evidence.
func wh004ParseCABundle(raw []byte) (certs []*x509.Certificate, unparseable bool) {
	rest := raw
	sawAnyBlock := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		sawAnyBlock = true
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		certs = append(certs, cert)
	}
	return certs, !sawAnyBlock
}

func wh004Finding(in wh004Input, ref findings.ResourceReference, severity findings.Severity, global bool, discriminator, targetVersion, message string, evidence []string, remediation string) findings.Finding {
	return findings.Finding{
		RuleID:        "WH-004",
		Severity:      severity,
		Confidence:    findings.TierStaticCertain,
		Message:       message,
		Resources:     []findings.ResourceReference{ref},
		Evidence:      append(evidence, fmt.Sprintf("failurePolicy: %s", failurePolicyLiteral(in.FailurePolicy))),
		Remediation:   remediation,
		GlobalBlocker: global,
		Fingerprint:   findings.FingerprintV2("WH-004", targetVersion, in.WebhookName+":"+discriminator, ref),
	}
}

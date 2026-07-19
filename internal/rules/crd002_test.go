package rules

import (
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func crd002CRD(name string, conversion *apiextensionsv1.CustomResourceConversion) apiextensionsv1.CustomResourceDefinition {
	return apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID("uid-" + name)},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Conversion: conversion},
	}
}

func crd002HealthyWebhookConversion(namespace, service string) *apiextensionsv1.CustomResourceConversion {
	return &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             &apiextensionsv1.WebhookClientConfig{Service: &apiextensionsv1.ServiceReference{Namespace: namespace, Name: service}},
			ConversionReviewVersions: []string{"v1"},
		},
	}
}

func crd002ReadyEndpointSlice(namespace, service string) discoveryv1.EndpointSlice {
	ready := true
	return discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      service + "-abcde",
			Labels:    map[string]string{"kubernetes.io/service-name": service},
		},
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: &ready},
		}},
	}
}

// crd002RequireFinding asserts fs has exactly count findings, all CRD-002
// Blockers.
func crd002RequireFinding(t *testing.T, fs []findings.Finding, count int) {
	t.Helper()
	if len(fs) != count {
		t.Fatalf("got %d findings, want %d: %+v", len(fs), count, fs)
	}
	for _, f := range fs {
		if f.RuleID != "CRD-002" {
			t.Errorf("RuleID = %q, want CRD-002", f.RuleID)
		}
		if f.Severity != findings.SeverityBlocker {
			t.Errorf("Severity = %q, want Blocker", f.Severity)
		}
	}
}

func TestCRD002_NoConversionOrNonWebhookStrategyIsClean(t *testing.T) {
	for name, conversion := range map[string]*apiextensionsv1.CustomResourceConversion{
		"nil conversion": nil,
		"None strategy":  {Strategy: apiextensionsv1.NoneConverter},
	} {
		t.Run(name, func(t *testing.T) {
			crd := crd002CRD("widgets.example.com", conversion)
			fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
			if err != nil || len(fs) != 0 {
				t.Fatalf("Evaluate() = %+v, %v; want no findings", fs, err)
			}
		})
	}
}

func TestCRD002_HealthyWebhookConversionIsClean(t *testing.T) {
	crd := crd002CRD("widgets.example.com", crd002HealthyWebhookConversion("operators", "widget-converter"))
	snap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
		Services:                  []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Namespace: "operators", Name: "widget-converter"}}},
		EndpointSlices:            []discoveryv1.EndpointSlice{crd002ReadyEndpointSlice("operators", "widget-converter")},
		Errors:                    map[string]error{},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings for a fully healthy conversion webhook", fs, err)
	}
}

func TestCRD002_StrategyWebhookWithNoWebhookConfig(t *testing.T) {
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.WebhookConverter, Webhook: nil})
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
	if fs[0].Confidence != findings.TierStaticCertain {
		t.Errorf("Confidence = %q, want STATIC_CERTAIN", fs[0].Confidence)
	}
}

func TestCRD002_MissingClientConfig(t *testing.T) {
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook:  &apiextensionsv1.WebhookConversion{ClientConfig: nil, ConversionReviewVersions: []string{"v1"}},
	})
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
}

func TestCRD002_ClientConfigWithNeitherServiceNorURL(t *testing.T) {
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook:  &apiextensionsv1.WebhookConversion{ClientConfig: &apiextensionsv1.WebhookClientConfig{}, ConversionReviewVersions: []string{"v1"}},
	})
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
}

func TestCRD002_URLBasedClientConfigSkipsServiceChecks(t *testing.T) {
	url := "https://converter.example.com/convert"
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook:  &apiextensionsv1.WebhookConversion{ClientConfig: &apiextensionsv1.WebhookClientConfig{URL: &url}, ConversionReviewVersions: []string{"v1"}},
	})
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 0 {
		t.Fatalf("Evaluate() = %+v, %v; want no findings -- URL-based webhooks aren't probed, only conversionReviewVersions is checked", fs, err)
	}
}

func TestCRD002_ReferencedServiceDoesNotExist(t *testing.T) {
	crd := crd002CRD("widgets.example.com", crd002HealthyWebhookConversion("operators", "widget-converter"))
	snap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
		// No matching Service in the snapshot at all.
		Errors: map[string]error{},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
	if got := fs[0].Message; !strings.Contains(got, "does not exist") {
		t.Errorf("Message = %q, want it to say the Service does not exist", got)
	}
}

func TestCRD002_ServiceExistsButHasNoReadyEndpoints(t *testing.T) {
	crd := crd002CRD("widgets.example.com", crd002HealthyWebhookConversion("operators", "widget-converter"))
	snap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
		Services:                  []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Namespace: "operators", Name: "widget-converter"}}},
		// No EndpointSlices at all -- Service exists, zero ready endpoints.
		Errors: map[string]error{},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
	if fs[0].Confidence != findings.TierObserved {
		t.Errorf("Confidence = %q, want OBSERVED for the live endpoint-health check", fs[0].Confidence)
	}
	if fs[0].RemediationDetail == nil || fs[0].RemediationDetail.VerifyCommand == "" {
		t.Error("expected the endpoint-health finding to carry a VerifyCommand, matching the original CRD-002 check")
	}
	wantFingerprint := findings.FingerprintV2("CRD-002", "1.34", "operators/widget-converter", fs[0].Resources[0])
	if fs[0].Fingerprint != wantFingerprint {
		t.Errorf("Fingerprint = %q, want %q (unchanged from before this rule was extended)", fs[0].Fingerprint, wantFingerprint)
	}
}

func TestCRD002_EmptyConversionReviewVersions(t *testing.T) {
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             &apiextensionsv1.WebhookClientConfig{Service: &apiextensionsv1.ServiceReference{Namespace: "operators", Name: "widget-converter"}},
			ConversionReviewVersions: nil,
		},
	})
	snap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
		Services:                  []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Namespace: "operators", Name: "widget-converter"}}},
		EndpointSlices:            []discoveryv1.EndpointSlice{crd002ReadyEndpointSlice("operators", "widget-converter")},
		Errors:                    map[string]error{},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
}

func TestCRD002_UnsupportedConversionReviewVersions(t *testing.T) {
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             &apiextensionsv1.WebhookClientConfig{Service: &apiextensionsv1.ServiceReference{Namespace: "operators", Name: "widget-converter"}},
			ConversionReviewVersions: []string{"v2beta1"},
		},
	})
	snap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
		Services:                  []corev1.Service{{ObjectMeta: metav1.ObjectMeta{Namespace: "operators", Name: "widget-converter"}}},
		EndpointSlices:            []discoveryv1.EndpointSlice{crd002ReadyEndpointSlice("operators", "widget-converter")},
		Errors:                    map[string]error{},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 1)
}

func TestCRD002_MultipleSimultaneousProblemsProduceDistinctFindings(t *testing.T) {
	crd := crd002CRD("widgets.example.com", &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig:             &apiextensionsv1.WebhookClientConfig{Service: &apiextensionsv1.ServiceReference{Namespace: "operators", Name: "widget-converter"}},
			ConversionReviewVersions: nil, // also broken
		},
	})
	snap := &k8s.Snapshot{
		CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
		// Service also missing entirely.
		Errors: map[string]error{},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	crd002RequireFinding(t, fs, 2)
	if fs[0].Fingerprint == fs[1].Fingerprint {
		t.Errorf("both findings share fingerprint %q, want distinct fingerprints per problem", fs[0].Fingerprint)
	}
}

func TestCRD002_UnavailableCollectorsAreSkipped(t *testing.T) {
	crd := crd002CRD("widgets.example.com", crd002HealthyWebhookConversion("operators", "widget-converter"))
	for _, key := range []string{"customresourcedefinitions", "endpointslices"} {
		t.Run(key, func(t *testing.T) {
			snap := &k8s.Snapshot{
				CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd},
				Errors:                    map[string]error{key: errors.New("collection failed")},
			}
			fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: snap}, "1.34")
			if err != nil || len(fs) != 0 {
				t.Fatalf("Evaluate() = %+v, %v; want no findings when %s collection failed", fs, err, key)
			}
		})
	}
}

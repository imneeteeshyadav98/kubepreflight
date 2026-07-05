package rules

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/collectors/k8s"
)

func TestCRD002_UnavailableConversionWebhook(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Conversion: &apiextensionsv1.CustomResourceConversion{Strategy: apiextensionsv1.WebhookConverter, Webhook: &apiextensionsv1.WebhookConversion{ClientConfig: &apiextensionsv1.WebhookClientConfig{Service: &apiextensionsv1.ServiceReference{Namespace: "operators", Name: "widget-converter"}}}}},
	}
	fs, err := (CRD002{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v; want one finding", fs, err)
	}
	if fs[0].RuleID != "CRD-002" {
		t.Fatalf("RuleID = %s", fs[0].RuleID)
	}
}

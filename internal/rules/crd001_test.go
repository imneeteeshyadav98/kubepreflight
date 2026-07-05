package rules

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubepreflight/internal/collectors/k8s"
)

func TestCRD001_LegacyStoredVersion(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Storage: true, Served: true}}},
		Status:     apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v1beta1", "v1"}},
	}
	fs, err := (CRD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v; want one finding", fs, err)
	}
}

func TestCRD001_CurrentStorageOnly(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Storage: true, Served: true}}},
		Status:     apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v1"}},
	}
	fs, _ := (CRD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if len(fs) != 0 {
		t.Fatalf("got %+v, want no findings", fs)
	}
}

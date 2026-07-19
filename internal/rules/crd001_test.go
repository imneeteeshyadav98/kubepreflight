package rules

import (
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func TestCRD001_StoredVersionMissingFromSpecBlocks(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Storage: true, Served: true}}},
		Status:     apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v1beta1", "v1"}},
	}
	fs, err := (CRD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v; want one finding", fs, err)
	}
	if fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("Severity = %s, want Blocker because v1beta1 is no longer served by spec.versions", fs[0].Severity)
	}
}

func TestCRD001_LegacyStoredVersionStillServedWarns(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
			{Name: "v1beta1", Served: true},
			{Name: "v1", Storage: true, Served: true},
		}},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v1beta1", "v1"}},
	}
	fs, err := (CRD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v; want one finding", fs, err)
	}
	if fs[0].Severity != findings.SeverityWarning {
		t.Fatalf("Severity = %s, want Warning while legacy version is still served", fs[0].Severity)
	}
}

func TestCRD001_StoredVersionPresentButNotServedBlocks(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
			{Name: "v1beta1", Served: false},
			{Name: "v1", Storage: true, Served: true},
		}},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v1beta1", "v1"}},
	}
	fs, err := (CRD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v; want one finding", fs, err)
	}
	if fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("Severity = %s, want Blocker when a stored version is not served", fs[0].Severity)
	}
	if !strings.Contains(strings.Join(fs[0].Evidence, "\n"), "unavailable stored version(s): v1beta1") {
		t.Fatalf("Evidence = %v, want unavailable stored version detail", fs[0].Evidence)
	}
}

func TestCRD001_BlockerPriorityThroughReport(t *testing.T) {
	crd := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "widgets.example.com", UID: "uid-crd"},
		Spec:       apiextensionsv1.CustomResourceDefinitionSpec{Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Storage: true, Served: true}}},
		Status:     apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: []string{"v1beta1", "v1"}},
	}
	fs, err := (CRD001{}).Evaluate(&ScanContext{K8s: &k8s.Snapshot{CustomResourceDefinitions: []apiextensionsv1.CustomResourceDefinition{crd}, Errors: map[string]error{}}}, "1.34")
	if err != nil || len(fs) != 1 {
		t.Fatalf("Evaluate() = %+v, %v; want one finding", fs, err)
	}
	r := findings.NewReport("1.34", "test", "", metav1.Now().Time, fs)
	f := r.Findings[0]
	if f.Priority != string(findings.PriorityP2) || f.AffectedScope != "workload" || f.CanUpgradeContinue {
		t.Fatalf("priority fields = %s/%s continue=%v, want P2/workload/false", f.Priority, f.AffectedScope, f.CanUpgradeContinue)
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

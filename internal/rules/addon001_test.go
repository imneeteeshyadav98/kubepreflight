package rules

import (
	"fmt"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	awscol "kubepreflight/internal/collectors/aws"
	k8scol "kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

func TestADDON001_Positive_IncompatibleVersion(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.15.0-eksbuild.1", CompatibleVersions: []string{"v1.18.0-eksbuild.1", "v1.18.1-eksbuild.1"}},
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}

	f := fs[0]
	if f.RuleID != "ADDON-001" {
		t.Errorf("RuleID = %q, want ADDON-001", f.RuleID)
	}
	if f.Severity != findings.SeverityBlocker {
		t.Errorf("Severity = %q, want Blocker", f.Severity)
	}
	if f.Confidence != findings.TierProviderReported {
		t.Errorf("Confidence = %q, want PROVIDER_REPORTED", f.Confidence)
	}
	if f.Resources[0].Name != "vpc-cni" {
		t.Errorf("resource name = %q, want vpc-cni", f.Resources[0].Name)
	}
	if !contains(f.Evidence, "compatibility status: incompatible") {
		t.Errorf("evidence = %v, want structured compatibility status", f.Evidence)
	}
	if !containsPrefix(f.Evidence, "required upgrade order: 1. Amazon VPC CNI") {
		t.Errorf("evidence = %v, want VPC CNI upgrade order", f.Evidence)
	}
}

func TestADDON001_Positive_NoCompatibleVersionAtAll(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "legacy-addon", CurrentVersion: "v0.1.0", CompatibleVersions: nil},
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
}

func TestADDON001_Negative_CompatibleVersionNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.18.1-eksbuild.1", CompatibleVersions: []string{"v1.18.0-eksbuild.1", "v1.18.1-eksbuild.1"}},
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 (current version is in the compatible list): %+v", len(fs), fs)
	}
}

func TestADDON001_CompatibleCoreDNSAndCSINoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "coredns", CurrentVersion: "v1.11.4-eksbuild.2", CompatibleVersions: []string{"v1.11.4-eksbuild.2"}},
			{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.44.0-eksbuild.1", CompatibleVersions: []string{"v1.44.0-eksbuild.1"}},
			{Name: "aws-efs-csi-driver", CurrentVersion: "v2.1.8-eksbuild.1", CompatibleVersions: []string{"v2.1.8-eksbuild.1"}},
		},
		Errors: map[string]error{},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 compatible CoreDNS/CSI add-ons: %+v", len(fs), fs)
	}
}

func TestADDON001_IncompatibleCoreDNSAndCSI(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "coredns", CurrentVersion: "v1.10.1-eksbuild.1", CompatibleVersions: []string{"v1.11.4-eksbuild.2"}},
			{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.30.0-eksbuild.1", CompatibleVersions: []string{"v1.44.0-eksbuild.1"}},
			{Name: "aws-efs-csi-driver", CurrentVersion: "v1.7.0-eksbuild.1", CompatibleVersions: []string{"v2.1.8-eksbuild.1"}},
		},
		Errors: map[string]error{},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 3 {
		t.Fatalf("got %d findings, want 3: %+v", len(fs), fs)
	}
	for _, f := range fs {
		if f.RuleID != "ADDON-001" || f.Severity != findings.SeverityBlocker {
			t.Fatalf("finding = %+v, want ADDON-001 blocker", f)
		}
		if !contains(f.Evidence, "target Kubernetes version: 1.34") || !contains(f.Evidence, "compatibility status: incompatible") {
			t.Errorf("evidence = %v, want target and incompatible status", f.Evidence)
		}
		if f.Fingerprint == "" || f.Fingerprint == "unavailable" {
			t.Errorf("fingerprint = %q, want deterministic fingerprint", f.Fingerprint)
		}
	}
}

func TestADDON001_Negative_NilAWSSnapshotNoFindingsNoError(t *testing.T) {
	sc := &ScanContext{AWS: nil}
	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate must not error when AWS enrichment was skipped: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when sc.AWS is nil: %+v", len(fs), fs)
	}
}

func TestADDON002_Positive_CriticalAddonCompatibilityUnknown(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{
			{Name: "vpc-cni", CurrentVersion: "v1.15.0-eksbuild.1", ClusterName: "prod"},
			{Name: "kube-proxy", CurrentVersion: "v1.29.0-eksbuild.1", ClusterName: "prod"},
			{Name: "coredns", CurrentVersion: "v1.10.1-eksbuild.1", ClusterName: "prod"},
			{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.30.0-eksbuild.1", ClusterName: "prod"},
			{Name: "aws-efs-csi-driver", CurrentVersion: "v1.7.0-eksbuild.1", ClusterName: "prod"},
		},
		Errors: map[string]error{
			"describe-addon-versions:vpc-cni":            fmt.Errorf("access denied"),
			"describe-addon-versions:kube-proxy":         fmt.Errorf("throttled"),
			"describe-addon-versions:coredns":            fmt.Errorf("access denied"),
			"describe-addon-versions:aws-ebs-csi-driver": fmt.Errorf("access denied"),
			"describe-addon-versions:aws-efs-csi-driver": fmt.Errorf("access denied"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 5 {
		t.Fatalf("got %d findings, want 5: %+v", len(fs), fs)
	}
	for _, f := range fs {
		if f.RuleID != "ADDON-002" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierProviderReported {
			t.Fatalf("finding identity = %+v, want ADDON-002 warning PROVIDER_REPORTED", f)
		}
		if !contains(f.Evidence, "compatibility status: unknown") {
			t.Errorf("evidence = %v, want unknown compatibility status", f.Evidence)
		}
		if !containsPrefix(f.Evidence, "confidence/source: AWS EKS add-on compatibility metadata unavailable") {
			t.Errorf("evidence = %v, want unavailable source", f.Evidence)
		}
		if !contains(f.Evidence, "target Kubernetes version: 1.34") {
			t.Errorf("evidence = %v, want target version", f.Evidence)
		}
		if f.Fingerprint == "" || f.Fingerprint == "unavailable" {
			t.Errorf("fingerprint = %q, want deterministic fingerprint", f.Fingerprint)
		}
	}
}

func TestADDON002_Positive_UnparseableCoreDNSVersionWarns(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "coredns", CurrentVersion: "", CompatibleVersions: []string{"v1.11.4-eksbuild.2"}}},
		Errors: map[string]error{},
	}}

	addon001, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(addon001) != 0 {
		t.Fatalf("ADDON001 got %d findings, want 0 when installed version is unknown: %+v", len(addon001), addon001)
	}

	addon002, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(addon002) != 1 {
		t.Fatalf("ADDON002 got %d findings, want 1: %+v", len(addon002), addon002)
	}
	if !containsPrefix(addon002[0].Evidence, "confidence/source: AWS EKS DescribeAddon did not provide an installed version") {
		t.Fatalf("evidence = %v, want installed-version source", addon002[0].Evidence)
	}
}

func TestADDON002_Negative_VerificationSucceededNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "kube-proxy", CurrentVersion: "v1.29.0-eksbuild.1", CompatibleVersions: []string{"v1.29.0-eksbuild.1"}}},
		Errors: map[string]error{},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when compatibility was verified: %+v", len(fs), fs)
	}
}

func TestADDON002_Negative_OptionalCSIDriverAbsentNoFinding(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "coredns", CurrentVersion: "v1.11.4-eksbuild.2", CompatibleVersions: []string{"v1.11.4-eksbuild.2"}}},
		Errors: map[string]error{
			"describe-addon-versions:aws-ebs-csi-driver": fmt.Errorf("not queried because not installed"),
			"describe-addon-versions:aws-efs-csi-driver": fmt.Errorf("not queried because not installed"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 when optional CSI drivers are absent from ListAddons: %+v", len(fs), fs)
	}
}

func TestADDON002_LiveMetricsServerAndIngressControllerUnverifiable(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("kube-system", "metrics-server", "uid-metrics", map[string]string{"k8s-app": "metrics-server"}, "registry.k8s.io/metrics-server/metrics-server:v0.7.2"),
			addonDeployment("ingress-nginx", "ingress-nginx-controller", "uid-ingress", map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, "registry.k8s.io/ingress-nginx/controller:v1.11.3"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(fs), fs)
	}
	byAddon := map[string]findings.Finding{}
	for _, f := range fs {
		addon := evidenceValue(f.Evidence, "installed add-on: ")
		byAddon[addon] = f
		if f.RuleID != "ADDON-002" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierObserved {
			t.Fatalf("finding = %+v, want ADDON-002 Warning OBSERVED", f)
		}
		if !contains(f.Evidence, "target Kubernetes version: 1.34") || !contains(f.Evidence, "compatibility status: unknown") {
			t.Errorf("evidence = %v, want target and unknown status", f.Evidence)
		}
		if !containsPrefix(f.Evidence, "confidence/source: live Kubernetes") {
			t.Errorf("evidence = %v, want live Kubernetes source", f.Evidence)
		}
		if f.Fingerprint == "" || f.Fingerprint == "unavailable" {
			t.Errorf("fingerprint = %q, want deterministic fingerprint", f.Fingerprint)
		}
	}
	if got := evidenceValue(byAddon["metrics-server"].Evidence, "installed version: "); got != "v0.7.2" {
		t.Fatalf("metrics-server installed version = %q, want v0.7.2", got)
	}
	if got := evidenceValue(byAddon["ingress-controller"].Evidence, "installed version: "); got != "v1.11.3" {
		t.Fatalf("ingress controller installed version = %q, want v1.11.3", got)
	}
}

func TestADDON002_LiveIngressDaemonSetNoDuplicateForSameResource(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		DaemonSets: []appsv1.DaemonSet{
			addonDaemonSet("ingress-nginx", "ingress-nginx-controller", "uid-ingress-ds", map[string]string{"app.kubernetes.io/name": "ingress-nginx", "app": "ingress-nginx"}, "registry.k8s.io/ingress-nginx/controller:v1.11.3"),
			addonDaemonSet("ingress-nginx", "ingress-nginx-controller", "uid-ingress-ds", map[string]string{"app.kubernetes.io/name": "ingress-nginx", "app": "ingress-nginx"}, "registry.k8s.io/ingress-nginx/controller:v1.11.3"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1 for duplicate same DaemonSet resource: %+v", len(fs), fs)
	}
	if fs[0].Resources[0].Kind != "DaemonSet" || fs[0].Resources[0].Name != "ingress-nginx-controller" {
		t.Fatalf("resource = %+v, want ingress-nginx DaemonSet", fs[0].Resources[0])
	}
}

func TestADDON002_LiveCertManagerAndExternalDNSUnverifiable(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("cert-manager", "cert-manager", "uid-cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "controller"}, "quay.io/jetstack/cert-manager-controller:v1.16.1"),
			addonDeployment("external-dns", "external-dns", "uid-external-dns", map[string]string{"app.kubernetes.io/name": "external-dns"}, "registry.k8s.io/external-dns/external-dns:v0.15.1"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(fs), fs)
	}
	byAddon := map[string]findings.Finding{}
	for _, f := range fs {
		addon := evidenceValue(f.Evidence, "installed add-on: ")
		byAddon[addon] = f
		if f.RuleID != "ADDON-002" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierObserved {
			t.Fatalf("finding = %+v, want ADDON-002 Warning OBSERVED", f)
		}
		if !contains(f.Evidence, "target Kubernetes version: 1.34") || !contains(f.Evidence, "compatibility status: unknown") {
			t.Errorf("evidence = %v, want target and unknown status", f.Evidence)
		}
		if !containsPrefix(f.Evidence, "confidence/source: live Kubernetes") {
			t.Errorf("evidence = %v, want live Kubernetes source", f.Evidence)
		}
		if f.Fingerprint == "" || f.Fingerprint == "unavailable" {
			t.Errorf("fingerprint = %q, want deterministic fingerprint", f.Fingerprint)
		}
	}
	if got := evidenceValue(byAddon["cert-manager"].Evidence, "installed version: "); got != "v1.16.1" {
		t.Fatalf("cert-manager installed version = %q, want v1.16.1", got)
	}
	if got := evidenceValue(byAddon["external-dns"].Evidence, "installed version: "); got != "v0.15.1" {
		t.Fatalf("external-dns installed version = %q, want v0.15.1", got)
	}
	if !containsPrefix(byAddon["cert-manager"].Evidence, "required upgrade order: 7. cert-manager") {
		t.Fatalf("cert-manager evidence = %v, want upgrade order", byAddon["cert-manager"].Evidence)
	}
	if !containsPrefix(byAddon["external-dns"].Evidence, "required upgrade order: 8. external-dns") {
		t.Fatalf("external-dns evidence = %v, want upgrade order", byAddon["external-dns"].Evidence)
	}
}

func TestADDON002_LiveCertManagerAuxiliaryDeploymentsDoNotDuplicate(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("cert-manager", "cert-manager", "uid-cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "controller"}, "quay.io/jetstack/cert-manager-controller:v1.16.1"),
			addonDeployment("cert-manager", "cert-manager-cainjector", "uid-cainjector", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "cainjector"}, "quay.io/jetstack/cert-manager-cainjector:v1.16.1"),
			addonDeployment("cert-manager", "cert-manager-webhook", "uid-webhook", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "webhook"}, "quay.io/jetstack/cert-manager-webhook:v1.16.1"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1 cert-manager controller finding only: %+v", len(fs), fs)
	}
	if got := evidenceValue(fs[0].Evidence, "installed add-on: "); got != "cert-manager" {
		t.Fatalf("installed add-on = %q, want cert-manager", got)
	}
	if fs[0].Resources[0].Name != "cert-manager" {
		t.Fatalf("resource = %+v, want cert-manager controller deployment", fs[0].Resources[0])
	}
}

func TestADDON002_LiveNoKnownAddonNoFinding(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("default", "api", "uid-api", map[string]string{"app": "api"}, "example.com/api:v1"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 without a known high-impact live add-on: %+v", len(fs), fs)
	}
}

func TestADDON001And002_ReportSemantics(t *testing.T) {
	blockers, err := (ADDON001{}).Evaluate(&ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "coredns", CurrentVersion: "v1.10.1-eksbuild.1", CompatibleVersions: []string{"v1.11.4-eksbuild.2"}}},
		Errors: map[string]error{},
	}}, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	r := findings.NewReport("1.34", "prod", "eks", time.Now(), blockers)
	if len(r.Findings) != 1 || r.Findings[0].Priority != string(findings.PriorityP4) || r.Findings[0].CanUpgradeContinue {
		t.Fatalf("ADDON-001 report finding = %+v, want P4 and canUpgradeContinue=false", r.Findings)
	}

	warnings, err := (ADDON002{}).Evaluate(&ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.30.0-eksbuild.1"}},
		Errors: map[string]error{"describe-addon-versions:aws-ebs-csi-driver": fmt.Errorf("access denied")},
	}}, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	r = findings.NewReport("1.34", "prod", "eks", time.Now(), warnings)
	if len(r.Findings) != 1 || r.Findings[0].Priority != string(findings.PriorityP3) || !r.Findings[0].CanUpgradeContinue {
		t.Fatalf("ADDON-002 report finding = %+v, want P3 and canUpgradeContinue=true", r.Findings)
	}
}

func addonDeployment(namespace, name, uid string, labels map[string]string, image string) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(uid), Labels: labels},
		Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: image}}},
		}},
	}
}

func addonDaemonSet(namespace, name, uid string, labels map[string]string, image string) appsv1.DaemonSet {
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(uid), Labels: labels},
		Spec: appsv1.DaemonSetSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: name, Image: image}}},
		}},
	}
}

func evidenceValue(values []string, prefix string) string {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimPrefix(value, prefix)
		}
	}
	return ""
}

func containsPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

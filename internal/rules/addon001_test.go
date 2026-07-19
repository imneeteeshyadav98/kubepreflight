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

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	k8scol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/k8s"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
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
			{Name: "vpc-cni", CurrentVersion: "v1.18.1-eksbuild.1", ClusterName: "prod"},
			{Name: "kube-proxy", CurrentVersion: "v1.34.0-eksbuild.1", ClusterName: "prod"},
			{Name: "coredns", CurrentVersion: "v1.11.4-eksbuild.2", ClusterName: "prod"},
			{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.44.0-eksbuild.1", ClusterName: "prod"},
			{Name: "aws-efs-csi-driver", CurrentVersion: "v2.1.8-eksbuild.1", ClusterName: "prod"},
		},
		Errors: map[string]error{
			"describe-addon-versions:vpc-cni":            fmt.Errorf("access denied"),
			"describe-addon-versions:kube-proxy":         fmt.Errorf("throttled"),
			"describe-addon-versions:coredns":            fmt.Errorf("access denied"),
			"describe-addon-versions:aws-ebs-csi-driver": fmt.Errorf("access denied"),
			"describe-addon-versions:aws-efs-csi-driver": fmt.Errorf("access denied"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.35")
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
		if !contains(f.Evidence, "catalog source: no catalog entry for provider=eks add-on target") {
			t.Errorf("evidence = %v, want missing catalog source", f.Evidence)
		}
		if !contains(f.Evidence, "target Kubernetes version: 1.35") {
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
	if !contains(addon002[0].Evidence, "compatibility status: unknown") {
		t.Fatalf("evidence = %v, want unknown catalog status", addon002[0].Evidence)
	}
	if !contains(addon002[0].Evidence, "minimum compatible version: v1.11.4-eksbuild.2") {
		t.Fatalf("evidence = %v, want catalog minimum", addon002[0].Evidence)
	}
	if !containsPrefix(addon002[0].Evidence, "catalog source: ") {
		t.Fatalf("evidence = %v, want catalog source", addon002[0].Evidence)
	}
}

func TestADDON002_CatalogUpgradeRecommendedWarnsP4(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "vpc-cni", CurrentVersion: "v1.18.0-eksbuild.1", ClusterName: "prod"}},
		Errors: map[string]error{},
	}}

	blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("ADDON001 got %d findings, want 0 for compatible-but-not-recommended version: %+v", len(blockers), blockers)
	}

	warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("ADDON002 got %d findings, want 1: %+v", len(warnings), warnings)
	}
	f := warnings[0]
	if f.RuleID != "ADDON-002" || f.Severity != findings.SeverityWarning {
		t.Fatalf("finding = %+v, want ADDON-002 warning", f)
	}
	if !contains(f.Evidence, "compatibility status: upgrade recommended") {
		t.Fatalf("evidence = %v, want upgrade recommended status", f.Evidence)
	}
	if !contains(f.Evidence, "minimum compatible version: v1.18.0-eksbuild.1") || !contains(f.Evidence, "recommended upgrade version: v1.18.1-eksbuild.1") {
		t.Fatalf("evidence = %v, want catalog minimum and recommendation", f.Evidence)
	}
	r := findings.NewReport("1.34", "prod", "eks", time.Now(), warnings)
	if len(r.Findings) != 1 || r.Findings[0].Priority != string(findings.PriorityP4) || !r.Findings[0].CanUpgradeContinue {
		t.Fatalf("ADDON-002 report finding = %+v, want P4 and canUpgradeContinue=true", r.Findings)
	}
}

func TestADDON001And002_CatalogIncompatibleMutuallyExclusive(t *testing.T) {
	sc := &ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "vpc-cni", CurrentVersion: "v1.15.0-eksbuild.1", ClusterName: "prod"}},
		Errors: map[string]error{"describe-addon-versions:vpc-cni": fmt.Errorf("access denied")},
	}}

	blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(blockers) != 1 || blockers[0].RuleID != "ADDON-001" {
		t.Fatalf("ADDON001 got %+v, want one ADDON-001 blocker", blockers)
	}
	warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ADDON002 got %d findings, want 0 for catalog-known incompatible add-on already owned by ADDON-001: %+v", len(warnings), warnings)
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

// Catalog thresholds under test (internal/compatcatalog/catalog.json, all
// provider=kubernetes kubernetesVersion=1.34 unless noted):
//   metrics-server:                min v0.7.2  recommended v0.7.2
//   ingress-nginx:                 min v1.10.0 recommended v1.12.0
//   cert-manager:                  min v1.15.0 recommended v1.16.1
//   external-dns:                  min v0.14.0 recommended v0.15.1
//   aws-load-balancer-controller:  min v2.8.0  recommended v2.9.0 (provider=eks)

func TestADDON002_LiveMetricsServerAndExternalDNSCompatibleNoFinding(t *testing.T) {
	// Both installed versions equal their catalog minimum AND recommended
	// exactly -- "known compatible/recommended -> no finding".
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("kube-system", "metrics-server", "uid-metrics", map[string]string{"k8s-app": "metrics-server"}, "registry.k8s.io/metrics-server/metrics-server:v0.7.2"),
			addonDeployment("external-dns", "external-dns", "uid-external-dns", map[string]string{"app.kubernetes.io/name": "external-dns"}, "registry.k8s.io/external-dns/external-dns:v0.15.1"),
		},
	}}

	blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("ADDON001 got %d findings, want 0: %+v", len(blockers), blockers)
	}
	warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ADDON002 got %d findings, want 0 for catalog-known compatible/recommended versions: %+v", len(warnings), warnings)
	}
}

func TestADDON002_LiveIngressNginxUpgradeRecommended(t *testing.T) {
	// v1.11.3 is above the v1.10.0 minimum but below the v1.12.0
	// recommendation -- "compatible but below recommended -> ADDON-002
	// Warning, P4".
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("ingress-nginx", "ingress-nginx-controller", "uid-ingress", map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, "registry.k8s.io/ingress-nginx/controller:v1.11.3"),
		},
	}}

	blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("ADDON001 got %d findings, want 0 for a compatible-but-not-recommended version: %+v", len(blockers), blockers)
	}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.RuleID != "ADDON-002" || f.Severity != findings.SeverityWarning || f.Confidence != findings.TierObserved {
		t.Fatalf("finding = %+v, want ADDON-002 Warning OBSERVED", f)
	}
	if !contains(f.Evidence, "compatibility status: upgrade recommended") {
		t.Errorf("evidence = %v, want upgrade recommended status", f.Evidence)
	}
	if !contains(f.Evidence, "minimum compatible version: v1.10.0") || !contains(f.Evidence, "recommended upgrade version: v1.12.0") {
		t.Errorf("evidence = %v, want catalog minimum and recommendation", f.Evidence)
	}
	r := findings.NewReport("1.34", "prod", "", time.Now(), fs)
	if len(r.Findings) != 1 || r.Findings[0].Priority != string(findings.PriorityP4) || !r.Findings[0].CanUpgradeContinue {
		t.Fatalf("ADDON-002 report finding = %+v, want P4 and canUpgradeContinue=true", r.Findings)
	}
}

func TestADDON001_LiveIngressNginxIncompatible(t *testing.T) {
	// v1.9.0 is below the v1.10.0 minimum -- "below known minimum ->
	// ADDON-001 Blocker, P2, canUpgradeContinue=false".
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("ingress-nginx", "ingress-nginx-controller", "uid-ingress", map[string]string{"app.kubernetes.io/name": "ingress-nginx"}, "registry.k8s.io/ingress-nginx/controller:v1.9.0"),
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	f := fs[0]
	if f.Severity != findings.SeverityBlocker || f.Confidence != findings.TierObserved {
		t.Fatalf("finding = %+v, want ADDON-001 Blocker OBSERVED", f)
	}
	if !contains(f.Evidence, "compatibility status: incompatible") {
		t.Errorf("evidence = %v, want incompatible status", f.Evidence)
	}
	r := findings.NewReport("1.34", "prod", "", time.Now(), fs)
	if len(r.Findings) != 1 || r.Findings[0].Priority != string(findings.PriorityP2) || r.Findings[0].CanUpgradeContinue {
		t.Fatalf("ADDON-001 report finding = %+v, want P2 and canUpgradeContinue=false", r.Findings)
	}

	warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ADDON002 got %d findings, want 0 for a live workload already owned by ADDON-001: %+v", len(warnings), warnings)
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

func TestADDON002_LiveCertManagerUpgradeRecommended(t *testing.T) {
	// v1.15.5 is above the v1.15.0 minimum but below the v1.16.1
	// recommendation.
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("cert-manager", "cert-manager", "uid-cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "controller"}, "quay.io/jetstack/cert-manager-controller:v1.15.5"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if !contains(fs[0].Evidence, "compatibility status: upgrade recommended") {
		t.Errorf("evidence = %v, want upgrade recommended status", fs[0].Evidence)
	}
	if !containsPrefix(fs[0].Evidence, "required upgrade order: 7. cert-manager") {
		t.Fatalf("cert-manager evidence = %v, want upgrade order", fs[0].Evidence)
	}
}

func TestADDON002_LiveCertManagerAuxiliaryDeploymentsDoNotDuplicate(t *testing.T) {
	// cert-manager's Helm chart deploys three separate Deployments across
	// three separate image repositories (controller, cainjector, webhook)
	// -- classifyLiveAddonByImage's strict image-repo matching must only
	// recognize the controller, not just avoid double-counting one
	// resource.
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("cert-manager", "cert-manager", "uid-cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "controller"}, "quay.io/jetstack/cert-manager-controller:v1.15.5"),
			addonDeployment("cert-manager", "cert-manager-cainjector", "uid-cainjector", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "cainjector"}, "quay.io/jetstack/cert-manager-cainjector:v1.15.5"),
			addonDeployment("cert-manager", "cert-manager-webhook", "uid-webhook", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "webhook"}, "quay.io/jetstack/cert-manager-webhook:v1.15.5"),
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

func TestADDON001_LiveCertManagerIncompatible(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("cert-manager", "cert-manager", "uid-cert-manager", map[string]string{"app.kubernetes.io/name": "cert-manager", "app.kubernetes.io/component": "controller"}, "quay.io/jetstack/cert-manager-controller:v1.14.0"),
		},
	}}

	fs, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
		t.Fatalf("got %+v, want one ADDON-001 blocker", fs)
	}
}

func TestADDON002_LiveUnparseableTagsStayUnverifiable(t *testing.T) {
	// "latest", a digest pin, and a purely non-numeric custom/fork tag all
	// fail compatcatalog.looksLikeVersion -- catalog lookup succeeds (a
	// real entry exists for external-dns) but InstalledStatus can't place
	// the installed version, so this must land on the "unknown" catalog
	// finding, not a false Compatible/Incompatible verdict. A custom tag
	// that DOES contain digits (e.g. "mycompany-patch-2") is a known,
	// accepted limitation shared with the EKS-managed add-on path and
	// compatcatalog itself (already established in PR #104/#105, not
	// something this change introduces or can fix without redesigning
	// CompareVersions's tokenizer): looksLikeVersion only requires *some*
	// digit to be present, so such a tag still gets a best-effort
	// comparison rather than falling back to "unknown".
	for _, tc := range []struct {
		name  string
		image string
	}{
		{"latest tag", "registry.k8s.io/external-dns/external-dns:latest"},
		{"digest only, no tag", "registry.k8s.io/external-dns/external-dns@sha256:1111111111111111111111111111111111111111111111111111111111aa"},
		{"custom fork build with no digits", "registry.k8s.io/external-dns/external-dns:mycompany-custom-build"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := &ScanContext{K8s: &k8scol.Snapshot{
				Deployments: []appsv1.Deployment{
					addonDeployment("external-dns", "external-dns", "uid-external-dns", map[string]string{"app.kubernetes.io/name": "external-dns"}, tc.image),
				},
			}}
			fs, err := (ADDON002{}).Evaluate(sc, "1.34")
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(fs) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
			}
			if !contains(fs[0].Evidence, "compatibility status: unknown") {
				t.Errorf("evidence = %v, want unknown compatibility status", fs[0].Evidence)
			}
			if !containsPrefix(fs[0].Evidence, "minimum compatible version: v0.14.0") {
				t.Errorf("evidence = %v, want the catalog minimum still surfaced even though the installed version is unparseable", fs[0].Evidence)
			}
		})
	}
}

func TestADDON002_LiveLegacyIngressControllerNoCatalogStillUnverifiable(t *testing.T) {
	// traefik has no compatibility catalog entry -- it must keep the
	// original always-unverifiable behavior via the name/label fallback,
	// not disappear or error just because ingress-nginx/ALB Controller now
	// have catalog coverage.
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("traefik", "traefik", "uid-traefik", map[string]string{"app.kubernetes.io/name": "traefik"}, "docker.io/traefik:v3.1.2"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if !contains(fs[0].Evidence, "compatibility status: unknown") {
		t.Errorf("evidence = %v, want unknown compatibility status (no catalog entry)", fs[0].Evidence)
	}
	if got := evidenceValue(fs[0].Evidence, "installed add-on: "); got != "ingress-controller" {
		t.Fatalf("installed add-on = %q, want the generic ingress-controller bucket", got)
	}
}

func TestADDON002_LiveUnrelatedWorkloadNamedLikeAnAddonDoesNotMatch(t *testing.T) {
	// "Image repository alias matching strict ho": a workload whose NAME
	// merely contains "ingress-nginx" but runs a completely unrelated
	// image must never be classified as the real ingress-nginx controller.
	sc := &ScanContext{K8s: &k8scol.Snapshot{
		Deployments: []appsv1.Deployment{
			addonDeployment("default", "my-ingress-nginx-test-harness", "uid-fake", map[string]string{"app": "my-ingress-nginx-test-harness"}, "example.com/internal-test-tool:v1"),
		},
	}}

	fs, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("got %d findings, want 0 -- a name that merely mentions ingress-nginx must not match without the real controller image: %+v", len(fs), fs)
	}
}

func TestADDON_LiveAWSLoadBalancerController(t *testing.T) {
	albDeployment := addonDeployment("kube-system", "aws-load-balancer-controller", "uid-alb", map[string]string{"app.kubernetes.io/name": "aws-load-balancer-controller"}, "public.ecr.aws/eks/aws-load-balancer-controller:v2.7.0")

	t.Run("incompatible when AWS enrichment is active", func(t *testing.T) {
		sc := &ScanContext{
			AWS: &awscol.Snapshot{},
			K8s: &k8scol.Snapshot{Deployments: []appsv1.Deployment{albDeployment}},
		}
		fs, err := (ADDON001{}).Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("ADDON001 Evaluate: %v", err)
		}
		if len(fs) != 1 || fs[0].Severity != findings.SeverityBlocker {
			t.Fatalf("got %+v, want one ADDON-001 blocker (v2.7.0 is below the v2.8.0 minimum)", fs)
		}
		if got := evidenceValue(fs[0].Evidence, "installed add-on: "); got != "aws-load-balancer-controller" {
			t.Fatalf("installed add-on = %q, want aws-load-balancer-controller", got)
		}

		warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("ADDON002 Evaluate: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("ADDON002 got %d findings, want 0 for a live workload already owned by ADDON-001: %+v", len(warnings), warnings)
		}
	})

	t.Run("compatible version, no finding, when AWS enrichment is active", func(t *testing.T) {
		compatible := addonDeployment("kube-system", "aws-load-balancer-controller", "uid-alb", map[string]string{"app.kubernetes.io/name": "aws-load-balancer-controller"}, "public.ecr.aws/eks/aws-load-balancer-controller:v2.9.0")
		sc := &ScanContext{
			AWS: &awscol.Snapshot{},
			K8s: &k8scol.Snapshot{Deployments: []appsv1.Deployment{compatible}},
		}
		blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("ADDON001 Evaluate: %v", err)
		}
		if len(blockers) != 0 {
			t.Fatalf("ADDON001 got %d findings, want 0: %+v", len(blockers), blockers)
		}
		warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("ADDON002 Evaluate: %v", err)
		}
		if len(warnings) != 0 {
			t.Fatalf("ADDON002 got %d findings, want 0 for a catalog-compatible/recommended version: %+v", len(warnings), warnings)
		}
	})

	t.Run("provider-specific EKS catalog data is not applied to a cluster-only scan", func(t *testing.T) {
		// Same incompatible v2.7.0 image as the first sub-test, but this
		// scan never confirmed AWS/EKS enrichment (sc.AWS is nil) -- the
		// "eks" catalog entry must not be trusted, so this stays an
		// ordinary unverifiable ADDON-002 warning instead of a false
		// ADDON-001 blocker.
		sc := &ScanContext{K8s: &k8scol.Snapshot{Deployments: []appsv1.Deployment{albDeployment}}}

		blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("ADDON001 Evaluate: %v", err)
		}
		if len(blockers) != 0 {
			t.Fatalf("ADDON001 got %d findings, want 0 without confirmed AWS/EKS enrichment: %+v", len(blockers), blockers)
		}

		warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
		if err != nil {
			t.Fatalf("ADDON002 Evaluate: %v", err)
		}
		if len(warnings) != 1 {
			t.Fatalf("ADDON002 got %d findings, want 1 unverifiable warning: %+v", len(warnings), warnings)
		}
		if !contains(warnings[0].Evidence, "compatibility status: unknown") {
			t.Errorf("evidence = %v, want the plain unverifiable path, not a catalog-backed verdict", warnings[0].Evidence)
		}
	})
}

func TestClassifyLiveAddonByImage(t *testing.T) {
	cases := []struct {
		name      string
		image     string
		wantAddon string
		wantOK    bool
	}{
		{"metrics-server via registry.k8s.io", "registry.k8s.io/metrics-server/metrics-server:v0.7.2", "metrics-server", true},
		{"ingress-nginx via registry.k8s.io", "registry.k8s.io/ingress-nginx/controller:v1.11.3", "ingress-nginx", true},
		{"ALB controller via public ECR", "public.ecr.aws/eks/aws-load-balancer-controller:v2.9.0", "aws-load-balancer-controller", true},
		{"ALB controller via amazon/ mirror", "docker.io/amazon/aws-load-balancer-controller:v2.9.0", "aws-load-balancer-controller", true},
		{"cert-manager controller via quay.io", "quay.io/jetstack/cert-manager-controller:v1.16.1", "cert-manager", true},
		{"external-dns via registry.k8s.io", "registry.k8s.io/external-dns/external-dns:v0.15.1", "external-dns", true},
		{"external-dns via bitnami mirror", "docker.io/bitnami/external-dns:0.15.1", "external-dns", true},
		{"cert-manager cainjector does not match the controller signature", "quay.io/jetstack/cert-manager-cainjector:v1.16.1", "", false},
		{"cert-manager webhook does not match the controller signature", "quay.io/jetstack/cert-manager-webhook:v1.16.1", "", false},
		{"digest pinned image still matches its repo", "registry.k8s.io/metrics-server/metrics-server@sha256:1111111111111111111111111111111111111111111111111111111111aa", "metrics-server", true},
		{"unrelated image with a similar-looking name is not fooled", "example.com/internal/fake-metrics-server-clone:v1", "", false},
		{"empty image", "", "", false},
		{"private mirror keeps the deterministic suffix", "my-private-registry.internal:5000/mirror/ingress-nginx/controller:v1.11.3", "ingress-nginx", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addon, ok := classifyLiveAddonByImage(tc.image)
			if ok != tc.wantOK {
				t.Fatalf("classifyLiveAddonByImage(%q) ok = %v, want %v", tc.image, ok, tc.wantOK)
			}
			if ok && addon != tc.wantAddon {
				t.Errorf("classifyLiveAddonByImage(%q) = %q, want %q", tc.image, addon, tc.wantAddon)
			}
		})
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

func TestADDON002_LiveAddonAbsentNoFinding(t *testing.T) {
	sc := &ScanContext{K8s: &k8scol.Snapshot{}}

	blockers, err := (ADDON001{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON001 Evaluate: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("ADDON001 got %d findings, want 0 with no workloads at all: %+v", len(blockers), blockers)
	}
	warnings, err := (ADDON002{}).Evaluate(sc, "1.34")
	if err != nil {
		t.Fatalf("ADDON002 Evaluate: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("ADDON002 got %d findings, want 0 with no workloads at all: %+v", len(warnings), warnings)
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
	if len(r.Findings) != 1 || r.Findings[0].Priority != string(findings.PriorityP2) || r.Findings[0].CanUpgradeContinue {
		t.Fatalf("ADDON-001 report finding = %+v, want P2 and canUpgradeContinue=false", r.Findings)
	}

	warnings, err := (ADDON002{}).Evaluate(&ScanContext{AWS: &awscol.Snapshot{
		Addons: []awscol.AddonRecord{{Name: "aws-ebs-csi-driver", CurrentVersion: "v1.44.0-eksbuild.1"}},
		Errors: map[string]error{},
	}}, "1.35")
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

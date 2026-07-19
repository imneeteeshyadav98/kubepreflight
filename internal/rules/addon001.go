package rules

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	awscol "github.com/imneeteeshyadav98/kubepreflight/internal/collectors/aws"
	"github.com/imneeteeshyadav98/kubepreflight/internal/compatcatalog"
	"github.com/imneeteeshyadav98/kubepreflight/internal/exemptions"
	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// ADDON001 flags an installed EKS add-on whose currently-installed version
// is not among AWS's own reported set of versions compatible with the
// scan's target Kubernetes version — a deterministic preflight check
// queryable before the upgrade even starts (deep dive Section 9, check
// ADDON-001).
type ADDON001 struct{}

func (ADDON001) ID() string { return "ADDON-001" }

func (ADDON001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil {
		return nil, nil
	}

	var out []findings.Finding
	if sc.AWS != nil {
		for _, addon := range sc.AWS.Addons {
			if isCatalogManagedEKSAddon(addon.Name) {
				entry, ok := lookupEKSAddonCatalog(addon.Name, targetVersion)
				if !ok {
					continue
				}
				if entry.InstalledStatus(addon.CurrentVersion) == compatcatalog.StatusIncompatible {
					out = append(out, addon001CatalogFinding(addon, targetVersion, entry))
				}
				continue
			}
			if _, unavailable := sc.AWS.Errors["describe-addon-versions:"+addon.Name]; unavailable {
				continue
			}
			if strings.TrimSpace(addon.CurrentVersion) == "" {
				continue
			}
			if isVersionCompatible(addon.CurrentVersion, addon.CompatibleVersions) {
				continue
			}
			out = append(out, addon001Finding(addon, targetVersion))
		}
	}
	// Live workload add-ons (metrics-server, ingress-nginx, AWS Load
	// Balancer Controller, cert-manager, external-dns) are independent of
	// AWS enrichment -- they're detected from live cluster state
	// (sc.K8s), not the AWS API, so this runs whether or not --provider=eks
	// was used. ADDON-002's live-workload loop below evaluates the exact
	// same classified workloads against the same catalog lookup and
	// explicitly skips StatusIncompatible/StatusCompatible, so a given
	// workload can never produce both an ADDON-001 and an ADDON-002
	// finding -- mirrors the EKS-managed add-on split above.
	if sc.K8s != nil {
		awsAvailable := sc.AWS != nil
		for _, addon := range liveUnverifiableAddons(sc.K8s.Deployments, sc.K8s.DaemonSets) {
			entry, ok := lookupLiveAddonCatalog(addon.addonName, targetVersion, awsAvailable)
			if !ok {
				continue // no catalog entry (or a provider-scoped entry this scan can't confirm) -- ADDON-002 owns that case
			}
			if entry.InstalledStatus(addon.installedVersion) == compatcatalog.StatusIncompatible {
				out = append(out, addon001LiveCatalogFinding(addon, targetVersion, entry))
			}
		}
	}
	return out, nil
}

func isVersionCompatible(current string, compatible []string) bool {
	for _, v := range compatible {
		if v == current {
			return true
		}
	}
	return false
}

// ADDON002 flags high-impact EKS add-ons whose target-version compatibility
// could not be verified. ADDON-001 owns known incompatibility; this rule owns
// the "unknown but important" state so it does not disappear into inventory.
type ADDON002 struct{}

func (ADDON002) ID() string { return "ADDON-002" }

func (ADDON002) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc == nil {
		return nil, nil
	}
	var out []findings.Finding
	if sc.AWS != nil {
		for _, addon := range sc.AWS.Addons {
			if !isHighImpactAddon(addon.Name) {
				continue
			}
			if isCatalogManagedEKSAddon(addon.Name) {
				entry, ok := lookupEKSAddonCatalog(addon.Name, targetVersion)
				if !ok {
					out = append(out, addon002CatalogMissingFinding(addon, targetVersion))
					continue
				}
				switch entry.InstalledStatus(addon.CurrentVersion) {
				case compatcatalog.StatusIncompatible, compatcatalog.StatusCompatible:
					continue
				case compatcatalog.StatusUpgradeRecommended:
					out = append(out, addon002CatalogUpgradeRecommendedFinding(addon, targetVersion, entry))
					continue
				default:
					out = append(out, addon002CatalogUnknownFinding(addon, targetVersion, entry))
					continue
				}
			}
			err, unavailable := addonVerificationError(addon, sc.AWS.Errors)
			if err == nil && !unavailable {
				continue
			}
			out = append(out, addon002Finding(addon, targetVersion, err))
		}
	}
	if sc.K8s != nil {
		awsAvailable := sc.AWS != nil
		for _, addon := range liveUnverifiableAddons(sc.K8s.Deployments, sc.K8s.DaemonSets) {
			entry, ok := lookupLiveAddonCatalog(addon.addonName, targetVersion, awsAvailable)
			if !ok {
				out = append(out, addon002LiveFinding(addon, targetVersion))
				continue
			}
			switch entry.InstalledStatus(addon.installedVersion) {
			case compatcatalog.StatusIncompatible, compatcatalog.StatusCompatible:
				continue // StatusIncompatible is ADDON-001's; StatusCompatible needs no finding at all
			case compatcatalog.StatusUpgradeRecommended:
				out = append(out, addon002LiveCatalogUpgradeRecommendedFinding(addon, targetVersion, entry))
			default:
				out = append(out, addon002LiveCatalogUnknownFinding(addon, targetVersion, entry))
			}
		}
	}
	return out, nil
}

func isHighImpactAddon(name string) bool {
	switch name {
	case "vpc-cni", "kube-proxy", "coredns", "aws-ebs-csi-driver", "aws-efs-csi-driver":
		return true
	default:
		return false
	}
}

func isCatalogManagedEKSAddon(name string) bool {
	switch name {
	case "vpc-cni", "kube-proxy", "coredns", "aws-ebs-csi-driver", "aws-efs-csi-driver":
		return true
	default:
		return false
	}
}

func lookupEKSAddonCatalog(addonName, targetVersion string) (compatcatalog.Entry, bool) {
	catalog, err := compatcatalog.Default()
	if err != nil {
		return compatcatalog.Entry{}, false
	}
	return catalog.Lookup("eks", addonName, targetVersion)
}

// liveAddonCatalogProvider maps a live-workload add-on name to the
// compatibility catalog provider scope its entry belongs to, and whether
// that scope requires this scan to have confirmed AWS/EKS enrichment.
// metrics-server, ingress-nginx, cert-manager, and external-dns are
// ordinary Kubernetes software that runs identically on any cluster type,
// so they use the generic "kubernetes" provider and apply everywhere. AWS
// Load Balancer Controller only makes sense on EKS, so it's catalogued
// under "eks" and requiresAWS=true -- provider-specific catalog data must
// never be applied to a cluster this scan hasn't actually confirmed is
// that provider (a cluster-only scan that happens to have an ALB
// Controller-shaped Deployment installed, e.g. self-managed Kubernetes on
// EC2 mimicking the same controller, must not silently borrow EKS
// version-compatibility facts).
func liveAddonCatalogProvider(addonName string) (provider string, requiresAWS bool) {
	switch addonName {
	case "aws-load-balancer-controller":
		return "eks", true
	case "metrics-server", "ingress-nginx", "cert-manager", "external-dns":
		return "kubernetes", false
	default:
		return "", false
	}
}

func lookupLiveAddonCatalog(addonName, targetVersion string, awsAvailable bool) (compatcatalog.Entry, bool) {
	provider, requiresAWS := liveAddonCatalogProvider(addonName)
	if provider == "" {
		return compatcatalog.Entry{}, false
	}
	if requiresAWS && !awsAvailable {
		if exemptions.MustGet(exemptions.AddonProviderScopedCatalogID).EvaluationPlane != exemptions.PlaneLive {
			return compatcatalog.Entry{}, false
		}
		return compatcatalog.Entry{}, false
	}
	catalog, err := compatcatalog.Default()
	if err != nil {
		return compatcatalog.Entry{}, false
	}
	return catalog.Lookup(provider, addonName, targetVersion)
}

func addonVerificationError(addon awscol.AddonRecord, errs map[string]error) (error, bool) {
	if err, unavailable := errs["describe-addon-versions:"+addon.Name]; unavailable {
		return err, true
	}
	if err, unavailable := errs["describe-addon:"+addon.Name]; unavailable {
		return err, true
	}
	if strings.TrimSpace(addon.CurrentVersion) == "" {
		return fmt.Errorf("installed add-on version was not reported by AWS DescribeAddon"), true
	}
	return nil, false
}

func addon001Finding(addon awscol.AddonRecord, targetVersion string) findings.Finding {
	var msg string
	if len(addon.CompatibleVersions) == 0 {
		msg = fmt.Sprintf(
			"EKS add-on %q version %s: AWS reports no compatible version of this add-on for target Kubernetes %s — it must be upgraded, replaced, or removed before upgrading",
			addon.Name, addon.CurrentVersion, targetVersion)
	} else {
		msg = fmt.Sprintf(
			"EKS add-on %q is on version %s, which is not in AWS's list of versions compatible with target Kubernetes %s",
			addon.Name, addon.CurrentVersion, targetVersion)
	}

	remediation := "Choose an AWS-reported compatible add-on version, review the add-on's current customizations, and update it in the provider-recommended sequence. "
	if len(addon.CompatibleVersions) > 0 {
		remediation += fmt.Sprintf("Compatible versions for target %s: %s. ", targetVersion, strings.Join(addon.CompatibleVersions, ", "))
	}
	remediation += "Upgrade order: validate CNI first, then kube-proxy, then DNS/storage/other add-ons. "
	remediation += "Confirm which fields are customized before choosing --resolve-conflicts: OVERWRITE silently destroys customizations, " +
		"PRESERVE keeps them but can fail the update, NONE fails on any conflict."

	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	required := "a version compatible with the target"
	if len(addon.CompatibleVersions) > 0 {
		required = strings.Join(addon.CompatibleVersions, " or ")
	}
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on version", Current: addon.CurrentVersion, Required: required}},
		SafeFix: &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Review add-on customizations and choose a compatible version before updating; PRESERVE can fail on conflicts while OVERWRITE can destroy customizations."}, Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion))},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("installed add-on: %s", addon.Name),
			fmt.Sprintf("current version: %s", addon.CurrentVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			fmt.Sprintf("minimum supported version: %s", required),
			fmt.Sprintf("AWS-reported compatible versions: %s", strings.Join(addon.CompatibleVersions, ", ")),
			fmt.Sprintf("recommended upgrade version: %s", required),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
			"compatibility status: incompatible",
			"confidence/source: AWS EKS DescribeAddonVersions",
		},
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-001", targetVersion, "", ref),
	}
}

func addon001CatalogFinding(addon awscol.AddonRecord, targetVersion string, entry compatcatalog.Entry) findings.Finding {
	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	msg := fmt.Sprintf(
		"EKS add-on %q is on version %s, which is below the catalog minimum %s for target Kubernetes %s",
		addon.Name, addon.CurrentVersion, entry.MinimumCompatibleVersion, targetVersion)
	remediation := fmt.Sprintf(
		"Upgrade %s to at least %s before upgrading Kubernetes to %s. Recommended version: %s. Upgrade order: %s. Source: %s.",
		addon.Name, entry.MinimumCompatibleVersion, targetVersion, entry.RecommendedVersion, addonUpgradeOrder(addon.Name), entry.Source)
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on version", Current: addon.CurrentVersion, Required: ">=" + entry.MinimumCompatibleVersion}},
		SafeFix: &findings.RemediationAction{
			Label:   "Safe fix",
			Steps:   []string{"Review add-on customizations and update to a catalog-compatible version before upgrading Kubernetes."},
			Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion)),
		},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: append(catalogEvidence(addon, targetVersion, entry, "incompatible"),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
		),
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-001", targetVersion, "", ref),
	}
}

func addon002Finding(addon awscol.AddonRecord, targetVersion string, err error) findings.Finding {
	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	msg := fmt.Sprintf(
		"EKS add-on %q version %s could not be verified against target Kubernetes %s — confirm compatibility before starting the upgrade",
		addon.Name, addon.CurrentVersion, targetVersion)
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on compatibility", Current: "unknown", Required: "verified compatible with target Kubernetes " + targetVersion}},
		SafeFix: &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Restore DescribeAddonVersions access or manually verify the add-on version against the EKS compatibility matrix before upgrading."}, Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion))},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("installed add-on: %s", addon.Name),
			fmt.Sprintf("current version: %s", addon.CurrentVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			"minimum supported version: unknown",
			"compatibility status: unknown",
			fmt.Sprintf("confidence/source: %s", addon002Source(addon, err)),
			fmt.Sprintf("verification error: %v", err),
			fmt.Sprintf("recommended upgrade version: verify with AWS before upgrading %s", addon.Name),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
		},
		Remediation:       "Verify the add-on against AWS's target-version compatibility data before upgrading. Treat VPC CNI and kube-proxy as early-order add-ons because networking and service proxy behavior underpin the rest of the cluster.",
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-002", targetVersion, "", ref),
	}
}

func addon002CatalogUpgradeRecommendedFinding(addon awscol.AddonRecord, targetVersion string, entry compatcatalog.Entry) findings.Finding {
	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	msg := fmt.Sprintf(
		"EKS add-on %q version %s is compatible with target Kubernetes %s but is below the catalog recommended version %s",
		addon.Name, addon.CurrentVersion, targetVersion, entry.RecommendedVersion)
	return addon002CatalogFinding(addon, targetVersion, entry, ref, msg, "upgrade recommended",
		fmt.Sprintf("Update %s to the catalog recommended version %s before or during the add-on validation phase. This is non-blocking because the installed version meets the known minimum %s.", addon.Name, entry.RecommendedVersion, entry.MinimumCompatibleVersion))
}

func addon002CatalogUnknownFinding(addon awscol.AddonRecord, targetVersion string, entry compatcatalog.Entry) findings.Finding {
	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	msg := fmt.Sprintf(
		"EKS add-on %q version %s could not be parsed against the compatibility catalog for target Kubernetes %s — confirm compatibility before starting the upgrade",
		addon.Name, addon.CurrentVersion, targetVersion)
	return addon002CatalogFinding(addon, targetVersion, entry, ref, msg, "unknown",
		"Verify the installed add-on version against the catalog source before upgrading. Unknown or custom add-on builds remain non-blocking warnings until compatibility is confirmed.")
}

func addon002CatalogFinding(addon awscol.AddonRecord, targetVersion string, entry compatcatalog.Entry, ref findings.ResourceReference, msg, status, remediation string) findings.Finding {
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on compatibility", Current: status, Required: "compatible with target Kubernetes " + targetVersion}},
		SafeFix: &findings.RemediationAction{
			Label:   "Safe fix",
			Steps:   []string{"Review the catalog source and add-on customizations before updating."},
			Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion)),
		},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: append(catalogEvidence(addon, targetVersion, entry, status),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
		),
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-002", targetVersion, "", ref),
	}
}

func addon002CatalogMissingFinding(addon awscol.AddonRecord, targetVersion string) findings.Finding {
	ref := findings.AWSResource("EKSAddon", addon.Name, addon.Name)
	msg := fmt.Sprintf(
		"EKS add-on %q version %s has no compatibility catalog entry for target Kubernetes %s — confirm compatibility before starting the upgrade",
		addon.Name, addon.CurrentVersion, targetVersion)
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on compatibility", Current: "unknown", Required: "catalog-backed compatibility for target Kubernetes " + targetVersion}},
		SafeFix: &findings.RemediationAction{Label: "Safe fix", Steps: []string{"Verify the add-on version against provider compatibility metadata before upgrading."}, Command: fmt.Sprintf("aws eks describe-addon-versions --addon-name %s --kubernetes-version %s", shellQuote(addon.Name), shellQuote(targetVersion))},
	}
	if addon.ClusterName != "" {
		detail.VerifyCommand = fmt.Sprintf("aws eks describe-addon --cluster-name %s --addon-name %s", shellQuote(addon.ClusterName), shellQuote(addon.Name))
	}
	return findings.Finding{
		RuleID:     "ADDON-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierProviderReported,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("installed add-on: %s", addon.Name),
			fmt.Sprintf("current version: %s", addon.CurrentVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			"minimum compatible version: unknown",
			"recommended upgrade version: unknown",
			"compatibility status: unknown",
			"catalog source: no catalog entry for provider=eks add-on target",
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.Name)),
		},
		Remediation:       "Verify this EKS managed add-on against provider compatibility metadata before upgrading. Missing catalog coverage is a warning, not proof of incompatibility.",
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-002", targetVersion, "", ref),
	}
}

func catalogEvidence(addon awscol.AddonRecord, targetVersion string, entry compatcatalog.Entry, status string) []string {
	return []string{
		fmt.Sprintf("installed add-on: %s", addon.Name),
		fmt.Sprintf("current version: %s", addon.CurrentVersion),
		fmt.Sprintf("target Kubernetes version: %s", targetVersion),
		fmt.Sprintf("minimum compatible version: %s", entry.MinimumCompatibleVersion),
		fmt.Sprintf("recommended upgrade version: %s", entry.RecommendedVersion),
		fmt.Sprintf("compatibility status: %s", status),
		fmt.Sprintf("catalog source: %s", entry.Source),
		fmt.Sprintf("catalog reference: %s", entry.Reference),
		fmt.Sprintf("catalog last verified date: %s", entry.LastVerifiedDate),
		fmt.Sprintf("catalog confidence: %s", entry.Confidence),
	}
}

type liveAddonWorkload struct {
	addonName        string
	kind             string
	namespace        string
	name             string
	uid              string
	installedVersion string
	image            string
	source           string
}

func liveUnverifiableAddons(deployments []appsv1.Deployment, daemonSets []appsv1.DaemonSet) []liveAddonWorkload {
	var out []liveAddonWorkload
	seen := map[string]bool{}
	for _, d := range deployments {
		version, image := addonWorkloadVersion(d.Spec.Template.Spec)
		if addonName, ok := classifyLiveAddon(d.Name, d.Namespace, d.Labels, image); ok {
			key := "Deployment/" + d.Namespace + "/" + d.Name
			if !seen[key] {
				seen[key] = true
				out = append(out, liveAddonWorkload{
					addonName:        addonName,
					kind:             "Deployment",
					namespace:        d.Namespace,
					name:             d.Name,
					uid:              string(d.UID),
					installedVersion: version,
					image:            image,
					source:           "live Kubernetes Deployment image",
				})
			}
		}
	}
	for _, ds := range daemonSets {
		version, image := addonWorkloadVersion(ds.Spec.Template.Spec)
		if addonName, ok := classifyLiveAddon(ds.Name, ds.Namespace, ds.Labels, image); ok {
			key := "DaemonSet/" + ds.Namespace + "/" + ds.Name
			if !seen[key] {
				seen[key] = true
				out = append(out, liveAddonWorkload{
					addonName:        addonName,
					kind:             "DaemonSet",
					namespace:        ds.Namespace,
					name:             ds.Name,
					uid:              string(ds.UID),
					installedVersion: version,
					image:            image,
					source:           "live Kubernetes DaemonSet image",
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].addonName != out[j].addonName {
			return out[i].addonName < out[j].addonName
		}
		if out[i].namespace != out[j].namespace {
			return out[i].namespace < out[j].namespace
		}
		if out[i].kind != out[j].kind {
			return out[i].kind < out[j].kind
		}
		return out[i].name < out[j].name
	})
	return out
}

// addonImageRepoSignature is one container image repository path (the
// portion before the tag/digest) that deterministically identifies a
// known live-workload add-on backed by a compatibility catalog entry.
// Matching on the image repository -- not just the workload's
// name/namespace/labels -- is what makes this classification strict: a
// workload named "my-ingress-nginx-test" that isn't actually running the
// real ingress-nginx controller image never matches, and cert-manager's
// webhook/cainjector Deployments (published under different image
// repositories from the controller: cert-manager-webhook,
// cert-manager-cainjector) are naturally excluded rather than needing a
// separate "if component == webhook, skip" rule layered on top.
var addonImageRepoSignatures = []struct {
	addonName string
	suffixes  []string
}{
	{"metrics-server", []string{"metrics-server/metrics-server"}},
	{"ingress-nginx", []string{"ingress-nginx/controller"}},
	{"aws-load-balancer-controller", []string{"eks/aws-load-balancer-controller", "amazon/aws-load-balancer-controller"}},
	{"cert-manager", []string{"jetstack/cert-manager-controller", "cert-manager/cert-manager-controller"}},
	{"external-dns", []string{"external-dns/external-dns", "bitnami/external-dns"}},
}

// legacyIngressIdentityTokens covers ingress controllers this codebase has
// always recognized (as an always-unverifiable ADDON-002 inventory item)
// but that have no compatibility catalog entry and are out of scope for
// catalog-backed verification -- kept as a name/label-based fallback,
// separate from the strict image-repository signatures above, so adding
// catalog coverage for ingress-nginx/AWS Load Balancer Controller doesn't
// regress detection breadth for these.
var legacyIngressIdentityTokens = []string{"traefik", "haproxy-ingress", "kong-ingress"}

// classifyLiveAddon identifies a Deployment/DaemonSet as a known add-on
// workload. image (the same value addonWorkloadVersion already resolved)
// is checked first and is authoritative for every catalog-backed add-on:
// see addonImageRepoSignatures's doc comment for why this is what makes
// matching strict. Name/label-based matching only remains as a narrower
// fallback for legacyIngressIdentityTokens, which have no catalog entry to
// protect against a false-positive catalog verdict either way.
func classifyLiveAddon(name, namespace string, labels map[string]string, image string) (string, bool) {
	if addonName, ok := classifyLiveAddonByImage(image); ok {
		return addonName, true
	}

	identity := strings.ToLower(strings.Join([]string{
		name,
		namespace,
		labels["app.kubernetes.io/name"],
		labels["app.kubernetes.io/component"],
		labels["app"],
		labels["k8s-app"],
		labels["name"],
	}, " "))
	for _, token := range legacyIngressIdentityTokens {
		if strings.Contains(identity, token) {
			return "ingress-controller", true
		}
	}
	return "", false
}

// classifyLiveAddonByImage matches a container image's repository path
// (see imageRepo) against addonImageRepoSignatures. A signature matches
// when the repository is exactly the known suffix or ends with
// "/<suffix>" -- tolerating any registry host or mirror prefix
// (registry.k8s.io, a private ECR pull-through cache, ...) while still
// requiring the full, deterministic vendor path, not a loose substring.
func classifyLiveAddonByImage(image string) (string, bool) {
	repo := imageRepo(image)
	if repo == "" {
		return "", false
	}
	for _, sig := range addonImageRepoSignatures {
		for _, suffix := range sig.suffixes {
			if repo == suffix || strings.HasSuffix(repo, "/"+suffix) {
				return sig.addonName, true
			}
		}
	}
	return "", false
}

// addonWorkloadVersion picks the first container with a non-empty image
// that classifyLiveAddonByImage recognizes, falling back to the first
// non-empty image at all if none match a known add-on signature -- a Pod
// commonly runs sidecars (e.g. a kube-rbac-proxy next to the real
// controller), so the add-on's own container is not reliably index 0.
func addonWorkloadVersion(spec corev1.PodSpec) (version, image string) {
	var firstImage string
	for _, c := range spec.Containers {
		if strings.TrimSpace(c.Image) == "" {
			continue
		}
		if firstImage == "" {
			firstImage = c.Image
		}
		if _, ok := classifyLiveAddonByImage(c.Image); ok {
			return versionOrUnknown(imageTag(c.Image)), c.Image
		}
	}
	if firstImage == "" {
		return "unknown", ""
	}
	return versionOrUnknown(imageTag(firstImage)), firstImage
}

func versionOrUnknown(tag string) string {
	if tag == "" {
		return "unknown"
	}
	return tag
}

// imageRepo returns image's repository path (registry host + path, minus
// any ":tag" or "@digest" suffix) -- the portion addonImageRepoSignatures
// matches against.
func imageRepo(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if i := strings.LastIndex(image, "@"); i >= 0 {
		image = image[:i]
	}
	slash := strings.LastIndex(image, "/")
	colon := strings.LastIndex(image, ":")
	if colon > slash {
		image = image[:colon]
	}
	return image
}

func imageTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if i := strings.LastIndex(image, "@"); i >= 0 {
		image = image[:i]
	}
	slash := strings.LastIndex(image, "/")
	colon := strings.LastIndex(image, ":")
	if colon <= slash {
		return ""
	}
	return image[colon+1:]
}

func addon002LiveFinding(addon liveAddonWorkload, targetVersion string) findings.Finding {
	ref := findings.LiveResource(addon.kind, findings.ScopeNamespaced, addon.namespace, addon.name, addon.uid)
	msg := fmt.Sprintf(
		"%s %s/%s version %s could not be verified against target Kubernetes %s — confirm compatibility before starting the upgrade",
		addon.addonName, addon.namespace, addon.name, addon.installedVersion, targetVersion)
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on compatibility", Current: "unknown", Required: "verified compatible with target Kubernetes " + targetVersion}},
		SafeFix: &findings.RemediationAction{
			Label:   "Safe fix",
			Steps:   []string{"Check the controller's published Kubernetes compatibility matrix for the installed image version before upgrading."},
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[*].image}'", strings.ToLower(addon.kind), shellQuote(addon.name), shellQuote(addon.namespace)),
		},
		VerifyCommand: fmt.Sprintf("kubectl rollout status %s/%s -n %s", strings.ToLower(addon.kind), shellQuote(addon.name), shellQuote(addon.namespace)),
	}
	return findings.Finding{
		RuleID:     "ADDON-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("installed add-on: %s", addon.addonName),
			fmt.Sprintf("installed version: %s", addon.installedVersion),
			fmt.Sprintf("target Kubernetes version: %s", targetVersion),
			"minimum supported version: unknown",
			"compatibility status: unknown",
			fmt.Sprintf("confidence/source: %s; no provider compatibility metadata available", addon.source),
			fmt.Sprintf("controller image: %s", addon.image),
			fmt.Sprintf("recommended upgrade version: verify with the %s compatibility matrix", addon.addonName),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.addonName)),
		},
		Remediation:       "Verify this add-on's controller image against its published Kubernetes compatibility matrix before upgrading. Unknown live add-on compatibility is a warning, not a hard blocker, because no deterministic provider compatibility source was available.",
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-002", targetVersion, "", ref),
	}
}

// liveCatalogEvidence mirrors catalogEvidence's shape for a live-workload
// resource instead of an AWS-managed one, plus the controller image and
// discovery source fields addon002LiveFinding already surfaces -- status
// must be one of the exact literal strings the other catalog finding
// builders use ("incompatible", "upgrade recommended", "unknown"), not
// compatcatalog.Status's own hyphenated String() form, because
// findings.AssignPriority's addon002UpgradeRecommended check matches the
// literal evidence string "compatibility status: upgrade recommended".
func liveCatalogEvidence(addon liveAddonWorkload, targetVersion string, entry compatcatalog.Entry, status string) []string {
	return []string{
		fmt.Sprintf("installed add-on: %s", addon.addonName),
		fmt.Sprintf("installed version: %s", addon.installedVersion),
		fmt.Sprintf("target Kubernetes version: %s", targetVersion),
		fmt.Sprintf("minimum compatible version: %s", entry.MinimumCompatibleVersion),
		fmt.Sprintf("recommended upgrade version: %s", entry.RecommendedVersion),
		fmt.Sprintf("compatibility status: %s", status),
		fmt.Sprintf("catalog source: %s", entry.Source),
		fmt.Sprintf("catalog reference: %s", entry.Reference),
		fmt.Sprintf("catalog last verified date: %s", entry.LastVerifiedDate),
		fmt.Sprintf("catalog confidence: %s", entry.Confidence),
		fmt.Sprintf("controller image: %s", addon.image),
		fmt.Sprintf("confidence/source: %s", addon.source),
	}
}

func addon001LiveCatalogFinding(addon liveAddonWorkload, targetVersion string, entry compatcatalog.Entry) findings.Finding {
	ref := findings.LiveResource(addon.kind, findings.ScopeNamespaced, addon.namespace, addon.name, addon.uid)
	msg := fmt.Sprintf(
		"%s %s/%s is on version %s, which is below the catalog minimum %s for target Kubernetes %s",
		addon.addonName, addon.namespace, addon.name, addon.installedVersion, entry.MinimumCompatibleVersion, targetVersion)
	remediation := fmt.Sprintf(
		"Upgrade %s to at least %s before upgrading Kubernetes to %s. Recommended version: %s. Upgrade order: %s. Source: %s.",
		addon.addonName, entry.MinimumCompatibleVersion, targetVersion, entry.RecommendedVersion, addonUpgradeOrder(addon.addonName), entry.Source)
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on version", Current: addon.installedVersion, Required: ">=" + entry.MinimumCompatibleVersion}},
		SafeFix: &findings.RemediationAction{
			Label:   "Safe fix",
			Steps:   []string{"Review how this add-on was installed (Helm chart, raw manifest, operator) and upgrade it through that same mechanism to a catalog-compatible version before upgrading Kubernetes."},
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[*].image}'", strings.ToLower(addon.kind), shellQuote(addon.name), shellQuote(addon.namespace)),
		},
		VerifyCommand: fmt.Sprintf("kubectl rollout status %s/%s -n %s", strings.ToLower(addon.kind), shellQuote(addon.name), shellQuote(addon.namespace)),
	}
	return findings.Finding{
		RuleID:     "ADDON-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: append(liveCatalogEvidence(addon, targetVersion, entry, "incompatible"),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.addonName)),
		),
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-001", targetVersion, "", ref),
	}
}

func addon002LiveCatalogUpgradeRecommendedFinding(addon liveAddonWorkload, targetVersion string, entry compatcatalog.Entry) findings.Finding {
	ref := findings.LiveResource(addon.kind, findings.ScopeNamespaced, addon.namespace, addon.name, addon.uid)
	msg := fmt.Sprintf(
		"%s %s/%s version %s is compatible with target Kubernetes %s but is below the catalog recommended version %s",
		addon.addonName, addon.namespace, addon.name, addon.installedVersion, targetVersion, entry.RecommendedVersion)
	return addon002LiveCatalogFinding(addon, targetVersion, entry, ref, msg, "upgrade recommended",
		fmt.Sprintf("Update %s to the catalog recommended version %s before or during the add-on validation phase. This is non-blocking because the installed version meets the known minimum %s.", addon.addonName, entry.RecommendedVersion, entry.MinimumCompatibleVersion))
}

func addon002LiveCatalogUnknownFinding(addon liveAddonWorkload, targetVersion string, entry compatcatalog.Entry) findings.Finding {
	ref := findings.LiveResource(addon.kind, findings.ScopeNamespaced, addon.namespace, addon.name, addon.uid)
	msg := fmt.Sprintf(
		"%s %s/%s version %s could not be parsed against the compatibility catalog for target Kubernetes %s — confirm compatibility before starting the upgrade",
		addon.addonName, addon.namespace, addon.name, addon.installedVersion, targetVersion)
	return addon002LiveCatalogFinding(addon, targetVersion, entry, ref, msg, "unknown",
		"Verify the installed version against the catalog source before upgrading. An unparseable tag (e.g. \"latest\", a digest pin, or a custom/fork build) remains a non-blocking warning until compatibility is confirmed some other way.")
}

func addon002LiveCatalogFinding(addon liveAddonWorkload, targetVersion string, entry compatcatalog.Entry, ref findings.ResourceReference, msg, status, remediation string) findings.Finding {
	detail := &findings.RemediationDetail{
		Changes: []findings.RemediationChange{{Field: "add-on compatibility", Current: status, Required: "compatible with target Kubernetes " + targetVersion}},
		SafeFix: &findings.RemediationAction{
			Label:   "Safe fix",
			Steps:   []string{"Review the catalog source and the controller's own release notes before updating."},
			Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[*].image}'", strings.ToLower(addon.kind), shellQuote(addon.name), shellQuote(addon.namespace)),
		},
		VerifyCommand: fmt.Sprintf("kubectl rollout status %s/%s -n %s", strings.ToLower(addon.kind), shellQuote(addon.name), shellQuote(addon.namespace)),
	}
	return findings.Finding{
		RuleID:     "ADDON-002",
		Severity:   findings.SeverityWarning,
		Confidence: findings.TierObserved,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: append(liveCatalogEvidence(addon, targetVersion, entry, status),
			fmt.Sprintf("required upgrade order: %s", addonUpgradeOrder(addon.addonName)),
		),
		Remediation:       remediation,
		RemediationDetail: detail,
		Fingerprint:       findings.FingerprintV2("ADDON-002", targetVersion, "", ref),
	}
}

func addon002Source(addon awscol.AddonRecord, err error) string {
	if strings.TrimSpace(addon.CurrentVersion) == "" {
		return "AWS EKS DescribeAddon did not provide an installed version"
	}
	if err != nil {
		return "AWS EKS add-on compatibility metadata unavailable"
	}
	return "AWS EKS add-on compatibility could not be verified"
}

func addonUpgradeOrder(name string) string {
	switch name {
	case "vpc-cni":
		return "1. Amazon VPC CNI before kube-proxy and DNS/storage add-ons"
	case "kube-proxy":
		return "2. kube-proxy after VPC CNI and before CoreDNS/storage add-ons"
	case "coredns":
		return "3. CoreDNS after VPC CNI and kube-proxy, before storage CSI add-ons"
	case "aws-ebs-csi-driver":
		return "4. EBS CSI driver after networking/DNS add-ons and before storage workload validation"
	case "aws-efs-csi-driver":
		return "4. EFS CSI driver after networking/DNS add-ons and before storage workload validation"
	case "metrics-server":
		return "5. metrics-server after core networking, DNS, and storage add-ons"
	case "ingress-controller", "ingress-nginx", "aws-load-balancer-controller":
		return "6. ingress controllers after networking, DNS, storage, and metrics add-ons"
	case "cert-manager":
		return "7. cert-manager after ingress controller compatibility is verified and before certificate validation"
	case "external-dns":
		return "8. external-dns after ingress controller compatibility and DNS ownership validation"
	default:
		return "review provider-recommended order"
	}
}

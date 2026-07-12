package rules

import (
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	awscol "kubepreflight/internal/collectors/aws"
	"kubepreflight/internal/findings"
)

// ADDON001 flags an installed EKS add-on whose currently-installed version
// is not among AWS's own reported set of versions compatible with the
// scan's target Kubernetes version — a deterministic preflight check
// queryable before the upgrade even starts (deep dive Section 9, check
// ADDON-001).
type ADDON001 struct{}

func (ADDON001) ID() string { return "ADDON-001" }

func (ADDON001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	if sc.AWS == nil {
		return nil, nil // AWS enrichment wasn't attempted or was gracefully skipped.
	}

	var out []findings.Finding
	for _, addon := range sc.AWS.Addons {
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
			err, unavailable := addonVerificationError(addon, sc.AWS.Errors)
			if err == nil && !unavailable {
				continue
			}
			out = append(out, addon002Finding(addon, targetVersion, err))
		}
	}
	if sc.K8s != nil {
		for _, addon := range liveUnverifiableAddons(sc.K8s.Deployments, sc.K8s.DaemonSets) {
			out = append(out, addon002LiveFinding(addon, targetVersion))
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
		if addonName, ok := classifyLiveAddon(d.Name, d.Namespace, d.Labels); ok {
			version, image := addonWorkloadVersion(d.Spec.Template.Spec)
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
		if addonName, ok := classifyLiveAddon(ds.Name, ds.Namespace, ds.Labels); ok {
			version, image := addonWorkloadVersion(ds.Spec.Template.Spec)
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

func classifyLiveAddon(name, namespace string, labels map[string]string) (string, bool) {
	lowerName := strings.ToLower(name)
	lowerComponent := strings.ToLower(labels["app.kubernetes.io/component"])
	labelIdentity := strings.ToLower(strings.Join([]string{
		labels["app.kubernetes.io/name"],
		labels["app.kubernetes.io/component"],
		labels["app"],
		labels["k8s-app"],
		labels["name"],
	}, " "))
	identity := strings.ToLower(strings.Join([]string{
		name,
		namespace,
		labels["app.kubernetes.io/name"],
		labels["app.kubernetes.io/component"],
		labels["app"],
		labels["k8s-app"],
		labels["name"],
	}, " "))

	if strings.Contains(identity, "metrics-server") {
		return "metrics-server", true
	}
	if lowerName == "cert-manager" || (strings.Contains(identity, "cert-manager") && lowerComponent == "controller") {
		return "cert-manager", true
	}
	if lowerName == "external-dns" || strings.Contains(labelIdentity, "external-dns") {
		return "external-dns", true
	}
	for _, token := range []string{
		"ingress-nginx",
		"nginx-ingress",
		"aws-load-balancer-controller",
		"traefik",
		"haproxy-ingress",
		"kong-ingress",
	} {
		if strings.Contains(identity, token) {
			return "ingress-controller", true
		}
	}
	return "", false
}

func addonWorkloadVersion(spec corev1.PodSpec) (version, image string) {
	for _, c := range spec.Containers {
		if strings.TrimSpace(c.Image) == "" {
			continue
		}
		image = c.Image
		version = imageTag(c.Image)
		if version == "" {
			version = "unknown"
		}
		return version, image
	}
	return "unknown", ""
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
	case "ingress-controller":
		return "6. ingress controllers after networking, DNS, storage, and metrics add-ons"
	case "cert-manager":
		return "7. cert-manager after ingress controller compatibility is verified and before certificate validation"
	case "external-dns":
		return "8. external-dns after ingress controller compatibility and DNS ownership validation"
	default:
		return "review provider-recommended order"
	}
}

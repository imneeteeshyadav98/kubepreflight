package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"kubepreflight/internal/apicatalog"
	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/collectors/manifest"
	"kubepreflight/internal/findings"
)

// API001 flags objects at a deprecated Kubernetes API group/version that
// will no longer be served once the target version is reached. Once the
// API server removes a group/version entirely, kubectl apply fails,
// controllers crash-loop, and stale-rendered Helm releases break on
// upgrade (deep dive Section 4, check API-001).
//
// Two independent evidence planes feed this rule: live cluster objects
// (sc.K8s, Plane 2) and static manifests/rendered Helm charts (sc.Manifests,
// Plane 1). Exact Kind+Namespace+Name matches correlate into one finding
// while retaining both occurrence references. An omitted namespace for a
// namespaced manifest never matches: apply-time namespace is unknowable.
//
// Ruleset entries live in internal/apicatalog — adding a newly-removed API
// there is a data change, never a code change here.
type API001 struct{}

func (API001) ID() string { return "API-001" }

func (API001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	targetMajor, targetMinor, err := parseMajorMinor(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("API-001: invalid target version %q: %w", targetVersion, err)
	}

	var out []findings.Finding

	if sc.K8s != nil {
		for _, obj := range sc.K8s.DeprecatedAPIUsage {
			if !targetReachesRemoval(obj.RemovedInVersion, targetMajor, targetMinor) {
				continue
			}
			if isEphemeralEvent(obj.DeprecatedAPI) {
				// Nobody hand-authors or migrates an Event: it's emitted by
				// whatever client-go version the calling controller
				// happens to link, self-expires within about an hour, and
				// a real cluster can have hundreds of them at any moment.
				// Flagging each one as an individually-actionable Blocker
				// is noise, not signal — the actionable target (upgrading
				// the emitting controller) is already covered by API-001
				// firing on that controller's own deprecated API usage, if
				// any. Silently excluded, not even as Info.
				continue
			}
			out = append(out, api001LiveFinding(obj, targetVersion))
		}
	}

	if sc.Manifests != nil {
		for _, obj := range sc.Manifests.DeprecatedAPIUsage {
			if !targetReachesRemoval(obj.RemovedInVersion, targetMajor, targetMinor) {
				continue
			}
			out = append(out, api001ManifestFinding(obj, targetVersion))
		}
	}

	return mergeAPI001Findings(out), nil
}

// targetReachesRemoval reports whether the scan's target version has
// reached (or passed) the version that removed an API. A malformed
// ruleset entry shouldn't silently swallow every other finding, so a
// parse failure here just excludes that one entry rather than erroring
// the whole rule.
func targetReachesRemoval(removedInVersion string, targetMajor, targetMinor int) bool {
	removedMajor, removedMinor, err := parseMajorMinor(removedInVersion)
	if err != nil {
		return false
	}
	return targetMajor == removedMajor && targetMinor >= removedMinor
}

// isEphemeralEvent reports whether a deprecated-API catalog entry is the
// core Events API. Nobody authors or migrates an Event by hand — it's
// always emitted by whatever client-go version the calling controller
// links — so live Event objects are excluded from API-001 entirely rather
// than reported as individually-actionable Blockers. Manifest-plane Events
// (nobody writes Event YAML in practice) are deliberately left untouched by
// this check — see the Evaluate loop, which only applies it to sc.K8s.
func isEphemeralEvent(dep apicatalog.DeprecatedAPI) bool {
	return dep.Group == "events.k8s.io"
}

// isAutoManagedFlowControl reports whether a live object is one of
// kube-apiserver's own bootstrap flowcontrol.apiserver.k8s.io defaults
// (see k8s.DeprecatedAPIObject.AutoManaged) rather than a FlowSchema/
// PriorityLevelConfiguration a user actually created and owns. The API
// server recreates these on its own if deleted or modified, so treating
// one as an upgrade-blocking migration task the reader owns is wrong —
// but they're still worth surfacing (Info, not silently dropped like
// Events) since a cluster with a genuinely custom flowcontrol setup should
// still see its bootstrap objects listed.
func isAutoManagedFlowControl(obj k8s.DeprecatedAPIObject) bool {
	return obj.Group == "flowcontrol.apiserver.k8s.io" && obj.AutoManaged
}

func api001LiveFinding(obj k8s.DeprecatedAPIObject, targetVersion string) findings.Finding {
	gv := obj.Group + "/" + obj.Version
	resourceLabel := obj.Name
	if obj.Namespace != "" {
		resourceLabel = obj.Namespace + "/" + obj.Name
	}

	if isAutoManagedFlowControl(obj) {
		return api001AutoManagedFlowControlFinding(obj, gv, resourceLabel, targetVersion)
	}

	msg := fmt.Sprintf(
		"%s %q (apiVersion %s) still exists at a version removed in Kubernetes %s — target version %s will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright",
		obj.Kind, resourceLabel, gv, obj.RemovedInVersion, targetVersion)

	remediation := fmt.Sprintf("Migrate to %s before upgrading past %s. Update the source manifest and review the official version-specific field changes; an apiVersion-only edit is not always sufficient. "+
		"For Helm releases whose stored release manifest still references %s, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. "+
		"If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.",
		obj.Replacement, obj.RemovedInVersion, gv)

	ref := findings.LiveResource(obj.Kind, apiResourceScope(obj.Namespaced), obj.Namespace, obj.Name, obj.UID)
	return findings.Finding{
		RuleID:     "API-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("apiVersion: %s", gv),
			fmt.Sprintf("removed in: Kubernetes %s", obj.RemovedInVersion),
			fmt.Sprintf("target version: %s", targetVersion),
			"detected via: live cluster object",
		},
		Remediation:       remediation,
		RemediationDetail: api001RemediationDetail(gv, obj.ReplacementAPIVersion, "", targetVersion),
		Fingerprint:       findings.FingerprintV2("API-001", targetVersion, "", ref),
	}
}

// api001AutoManagedFlowControlFinding is the Info-severity variant for a
// kube-apiserver bootstrap FlowSchema/PriorityLevelConfiguration default:
// same evidence and fingerprint shape as the normal Blocker path, but
// honest that there's usually nothing for the reader to do — the API
// server recreates these at the new version's default apiVersion on its
// own. No RemediationDetail: there's no diff to show for an object the
// reader doesn't edit.
func api001AutoManagedFlowControlFinding(obj k8s.DeprecatedAPIObject, gv, resourceLabel, targetVersion string) findings.Finding {
	msg := fmt.Sprintf(
		"%s %q (apiVersion %s) is a kube-apiserver-managed default that still exists at a version removed in Kubernetes %s — apiserver-managed default object; usually no direct user action, since kube-apiserver recreates its own flowcontrol defaults at the version it currently serves",
		obj.Kind, resourceLabel, gv, obj.RemovedInVersion)

	ref := findings.LiveResource(obj.Kind, apiResourceScope(obj.Namespaced), obj.Namespace, obj.Name, obj.UID)
	return findings.Finding{
		RuleID:     "API-001",
		Severity:   findings.SeverityInfo,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("apiVersion: %s", gv),
			fmt.Sprintf("removed in: Kubernetes %s", obj.RemovedInVersion),
			fmt.Sprintf("target version: %s", targetVersion),
			"detected via: live cluster object",
			"apf.kubernetes.io/autoupdate-spec: true (kube-apiserver-managed default, reconciled automatically)",
		},
		Remediation: "No action needed for this specific object: kube-apiserver owns and recreates its own flowcontrol bootstrap defaults at whatever apiVersion it currently serves. " +
			"If this cluster has custom FlowSchema/PriorityLevelConfiguration objects beyond the defaults, verify those separately — only kube-apiserver's own bootstrap set (marked with the apf.kubernetes.io/autoupdate-spec annotation) is covered by this note.",
		Fingerprint: findings.FingerprintV2("API-001", targetVersion, "", ref),
	}
}

// api001RemediationDetail is shared by the live and manifest variants; it
// returns nil when the catalog entry has no direct apiVersion swap to show
// (e.g. PodSecurityPolicy), leaving RemediationDetail nil rather than
// fabricating a diff that doesn't exist.
func api001RemediationDetail(currentGV, replacementGV, sourcePath, targetVersion string) *findings.RemediationDetail {
	if replacementGV == "" {
		return nil
	}
	verify := fmt.Sprintf("kubepreflight scan --target-version %s", shellQuote(targetVersion))
	if chart, ok := strings.CutPrefix(sourcePath, "helm:"); ok {
		verify = fmt.Sprintf("kubepreflight scan --helm-chart %s --target-version %s", shellQuote(chart), shellQuote(targetVersion))
	} else if sourcePath != "" {
		verify = fmt.Sprintf("kubepreflight scan --manifests %s --target-version %s", shellQuote(filepath.Dir(sourcePath)), shellQuote(targetVersion))
	}
	return &findings.RemediationDetail{
		AffectedFile: sourcePath,
		Changes: []findings.RemediationChange{
			{Field: "apiVersion", Current: currentGV, Required: replacementGV},
		},
		Diff: fmt.Sprintf("- apiVersion: %s\n+ apiVersion: %s", currentGV, replacementGV),
		SafeFix: &findings.RemediationAction{Label: "Safe fix", Steps: []string{
			"Update the manifest or controller source-of-truth to the replacement API and review version-specific field changes; an apiVersion-only edit is not always sufficient.",
			"Use the suggested diff as a starting point, then validate the rendered object before applying it.",
		}},
		VerifyCommand: verify,
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func api001ManifestFinding(obj manifest.DeprecatedAPIObject, targetVersion string) findings.Finding {
	gv := obj.Group + "/" + obj.Version
	resourceLabel := obj.Name
	if obj.Namespace != "" {
		resourceLabel = obj.Namespace + "/" + obj.Name
	}

	msg := fmt.Sprintf(
		"%s %q (apiVersion %s) in %s uses an API version removed in Kubernetes %s — this manifest will fail to apply once the cluster reaches target %s",
		obj.Kind, resourceLabel, gv, obj.SourcePath, obj.RemovedInVersion, targetVersion)

	remediation := fmt.Sprintf("Migrate to %s before this manifest is ever applied to a cluster at or past %s. Update and validate the source manifest against the replacement schema. "+
		"For Helm charts, update the template itself — bumping the chart version alone doesn't help if the template source still emits the old apiVersion.",
		obj.Replacement, obj.RemovedInVersion)

	ref := findings.ManifestResource(obj.Kind, apiResourceScope(obj.Namespaced), obj.Namespace, obj.Name, obj.SourcePath)
	return findings.Finding{
		RuleID:     "API-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("apiVersion: %s", gv),
			fmt.Sprintf("removed in: Kubernetes %s", obj.RemovedInVersion),
			fmt.Sprintf("target version: %s", targetVersion),
			fmt.Sprintf("source: %s", obj.SourcePath),
		},
		Remediation:       remediation,
		RemediationDetail: api001RemediationDetail(gv, obj.ReplacementAPIVersion, obj.SourcePath, targetVersion),
		Fingerprint:       findings.FingerprintV2("API-001", targetVersion, "", ref),
	}
}

func apiResourceScope(namespaced bool) findings.ResourceScope {
	if namespaced {
		return findings.ScopeNamespaced
	}
	return findings.ScopeCluster
}

// mergeAPI001Findings collapses equal conceptual fingerprints while retaining
// every occurrence. This is intentionally rule-local: two distinct rules on
// the same resource remain distinct correlation evidence in the report.
func mergeAPI001Findings(in []findings.Finding) []findings.Finding {
	byFingerprint := make(map[string]int, len(in))
	out := make([]findings.Finding, 0, len(in))
	for _, f := range in {
		idx, exists := byFingerprint[f.Fingerprint]
		if !exists {
			byFingerprint[f.Fingerprint] = len(out)
			out = append(out, f)
			continue
		}

		merged := &out[idx]
		for _, ref := range f.Resources {
			if !hasOccurrence(merged.Resources, ref.OccurrenceKey()) {
				merged.Resources = append(merged.Resources, ref)
			}
		}
		merged.Evidence = appendUnique(merged.Evidence, f.Evidence...)
		if hasPlane(merged.Resources, findings.PlaneLive) && hasPlane(merged.Resources, findings.PlaneManifest) {
			merged.Evidence = appendUnique(merged.Evidence,
				"cross-plane match: exact Kind+Namespace+Name identity",
				"cross-plane matches assume supplied manifests target this cluster")
		}
	}
	return out
}

func hasOccurrence(refs []findings.ResourceReference, key string) bool {
	for _, ref := range refs {
		if ref.OccurrenceKey() == key {
			return true
		}
	}
	return false
}

func hasPlane(refs []findings.ResourceReference, plane findings.Plane) bool {
	for _, ref := range refs {
		if ref.Plane == plane {
			return true
		}
	}
	return false
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]bool, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = true
	}
	for _, value := range values {
		if !seen[value] {
			dst = append(dst, value)
			seen[value] = true
		}
	}
	return dst
}

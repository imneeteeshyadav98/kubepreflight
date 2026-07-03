package rules

import (
	"fmt"

	"kubepreflight/internal/collectors/k8s"
	"kubepreflight/internal/findings"
)

// API001 flags live objects still stored at a deprecated Kubernetes API
// group/version that will no longer be served once the target version is
// reached. Once the API server removes a group/version entirely,
// kubectl apply fails, controllers crash-loop, and stale-rendered Helm
// releases break on upgrade (deep dive Section 4, check API-001).
//
// Ruleset entries live in internal/apicatalog — adding a newly-removed API
// there is a data change, never a code change here.
type API001 struct{}

func (API001) ID() string { return "API-001" }

func (API001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	targetMajor, targetMinor, err := parseMajorMinor(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("API-001: invalid target version %q: %w", targetVersion, err)
	}

	var out []findings.Finding
	for _, obj := range snap.DeprecatedAPIUsage {
		removedMajor, removedMinor, err := parseMajorMinor(obj.RemovedInVersion)
		if err != nil {
			// A malformed ruleset entry shouldn't silently swallow every
			// other finding; skip just this one instance.
			continue
		}
		// The API is only a problem for this upgrade if the target version
		// has reached (or passed) the version that removed it.
		if targetMajor != removedMajor || targetMinor < removedMinor {
			continue
		}
		out = append(out, api001Finding(obj, targetVersion))
	}
	return out, nil
}

func api001Finding(obj k8s.DeprecatedAPIObject, targetVersion string) findings.Finding {
	gv := obj.Group + "/" + obj.Version
	resourceLabel := obj.Name
	if obj.Namespace != "" {
		resourceLabel = obj.Namespace + "/" + obj.Name
	}

	msg := fmt.Sprintf(
		"%s %q (apiVersion %s) still exists at a version removed in Kubernetes %s — target version %s will no longer serve this API, and kubectl apply/controller reconciliation for it will fail outright",
		obj.Kind, resourceLabel, gv, obj.RemovedInVersion, targetVersion)

	remediation := fmt.Sprintf("Migrate to %s before upgrading past %s. For manifests: `kubectl convert -f <file> --output-version <group>/<version>`. "+
		"For Helm releases whose stored release manifest still references %s, a chart bump alone isn't enough — the release's stored manifest must be migrated too (mapkubeapis-style fix) or `helm upgrade` will fail even with fixed templates. "+
		"If a controller/operator is the one writing this object, upgrading the controller itself is required — its compiled-in client code is the actual caller.",
		obj.Replacement, obj.RemovedInVersion, gv)

	return findings.Finding{
		RuleID:     "API-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resource: findings.Resource{
			Kind:      obj.Kind,
			Namespace: obj.Namespace,
			Name:      obj.Name,
			UID:       obj.UID,
		},
		Evidence: []string{
			fmt.Sprintf("apiVersion: %s", gv),
			fmt.Sprintf("removed in: Kubernetes %s", obj.RemovedInVersion),
			fmt.Sprintf("target version: %s", targetVersion),
		},
		Remediation: remediation,
		Fingerprint: findings.Fingerprint("API-001", obj.UID, targetVersion),
	}
}

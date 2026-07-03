package rules

import (
	"fmt"
	"strconv"
	"strings"

	"kubepreflight/internal/findings"
)

// maxSupportedSkew is the number of minor versions a kubelet may lag behind
// the target control-plane version under the standard Kubernetes version
// skew policy (n-3 in modern Kubernetes). A kubelet must never be newer
// than the control plane.
const maxSupportedSkew = 3

// NODE001 flags a Node whose kubelet version is outside the supported skew
// window for the target upgrade version: either newer than the target
// (never allowed) or more than n-3 minor versions behind it. Deferred
// control-plane upgrades compound this — each skipped bump narrows the
// legal skew window for the next one (deep dive Section 10, check
// NODE-001).
type NODE001 struct{}

func (NODE001) ID() string { return "NODE-001" }

func (NODE001) Evaluate(sc *ScanContext, targetVersion string) ([]findings.Finding, error) {
	snap := sc.K8s
	targetMajor, targetMinor, err := parseMajorMinor(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("NODE-001: invalid target version %q: %w", targetVersion, err)
	}

	var out []findings.Finding
	for _, node := range snap.Nodes {
		kubeletVersion := node.Status.NodeInfo.KubeletVersion
		kMajor, kMinor, err := parseMajorMinor(kubeletVersion)
		if err != nil {
			// Can't evaluate skew without a parseable version; skip rather
			// than risk a false positive on a malformed/unknown value.
			continue
		}

		switch {
		case kMajor != targetMajor:
			out = append(out, node001Finding(node.Name, string(node.UID), kubeletVersion, targetVersion,
				fmt.Sprintf("kubelet major version %d does not match target major version %d", kMajor, targetMajor)))
		case kMinor > targetMinor:
			out = append(out, node001Finding(node.Name, string(node.UID), kubeletVersion, targetVersion,
				fmt.Sprintf("kubelet minor version %d is newer than target minor version %d — kubelet must never lead the control plane", kMinor, targetMinor)))
		case targetMinor-kMinor > maxSupportedSkew:
			out = append(out, node001Finding(node.Name, string(node.UID), kubeletVersion, targetVersion,
				fmt.Sprintf("kubelet minor version %d is %d minor versions behind target minor version %d — exceeds the supported n-%d skew policy",
					kMinor, targetMinor-kMinor, targetMinor, maxSupportedSkew)))
		}
	}
	return out, nil
}

// parseMajorMinor extracts the major and minor version numbers from a
// Kubernetes-style version string (e.g. "v1.33.0-eks-1234567" or "1.34"),
// ignoring any patch/build suffix.
func parseMajorMinor(v string) (major, minor int, err error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("cannot parse major.minor from %q", v)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing major version from %q: %w", v, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parsing minor version from %q: %w", v, err)
	}
	return major, minor, nil
}

func node001Finding(nodeName, nodeUID, kubeletVersion, targetVersion, reason string) findings.Finding {
	msg := fmt.Sprintf("Node %q: kubelet version %s is outside the supported skew window for target version %s — %s",
		nodeName, kubeletVersion, targetVersion, reason)

	remediation := "Replace this node (managed node group rolling update, Karpenter Drift, or manual AMI bump) to pick up a kubelet " +
		"within the supported skew window before proceeding with the next control-plane minor version upgrade. " +
		"Deferred control-plane upgrades compound this: each skipped bump narrows the legal skew window for the next one."

	ref := findings.LiveResource("Node", findings.ScopeCluster, "", nodeName, nodeUID)
	return findings.Finding{
		RuleID:     "NODE-001",
		Severity:   findings.SeverityBlocker,
		Confidence: findings.TierStaticCertain,
		Message:    msg,
		Resources:  []findings.ResourceReference{ref},
		Evidence: []string{
			fmt.Sprintf("kubelet version: %s", kubeletVersion),
			fmt.Sprintf("target version: %s", targetVersion),
			reason,
		},
		Remediation: remediation,
		Fingerprint: findings.FingerprintV2("NODE-001", targetVersion, "", ref),
	}
}

package findings

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const SchemaVersion = "1.0"

type CoverageStatus string

const (
	CoverageComplete CoverageStatus = "complete"
	CoveragePartial  CoverageStatus = "partial"
	CoverageSkipped  CoverageStatus = "skipped"
)

type PlaneCoverage struct {
	Status CoverageStatus `json:"status"`
	Errors []string       `json:"errors,omitempty"`
}

type ScanCoverage struct {
	Kubernetes PlaneCoverage `json:"kubernetes"`
	AWS        PlaneCoverage `json:"aws"`
	Manifests  PlaneCoverage `json:"manifests"`
}

const CrossPlaneManifestAssumption = "Cross-plane matches assume supplied manifests target this cluster."

// Summary holds finding counts by severity for quick terminal/report headers.
type Summary struct {
	Blockers int `json:"blockers"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

// Report is the top-level findings.json document produced by a scan.
type Report struct {
	SchemaVersion      string       `json:"schemaVersion"`
	CurrentVersion     string       `json:"currentVersion,omitempty"`
	TargetVersion      string       `json:"targetVersion"`
	ClusterContext     string       `json:"clusterContext,omitempty"`
	Provider           string       `json:"provider,omitempty"` // "eks", or empty for a cluster-only scan
	ScannedAt          time.Time    `json:"scannedAt"`
	Assumptions        []string     `json:"assumptions,omitempty"`
	NamespaceAllowlist []string     `json:"namespaceAllowlist,omitempty"`
	Findings           []Finding    `json:"findings"`
	Summary            Summary      `json:"summary"`
	Coverage           ScanCoverage `json:"coverage"`
	// EKSCluster is nil for every non-EKS scan and for an EKS scan where
	// AWS enrichment was unavailable (no credentials, no permissions) —
	// its absence must never be treated as an upgrade blocker, only as
	// "this metadata wasn't available." Populated from the same
	// DescribeCluster call the AWS collector already makes for
	// ClusterVersion/VpcID/EndpointAccess; no new AWS permission needed.
	EKSCluster *EKSClusterInfo `json:"eksCluster,omitempty"`
	// EKSAddons is the full inventory of installed EKS-managed add-ons —
	// every one ListAddons returned, not just the ones ADDON-001 flagged.
	// A compatible add-on never produces a finding (nothing wrong to
	// report), so without this list it's invisible in the report; this is
	// purely additive visibility, not a new check. Nil for a non-EKS scan
	// or when AWS enrichment was unavailable.
	EKSAddons []EKSAddonInfo `json:"eksAddons,omitempty"`
	// EKSNodegroups is the full inventory of EKS managed node groups
	// returned by ListNodegroups. Self-managed node groups are not returned
	// by that AWS API and therefore are not represented here. Nil for a
	// non-EKS scan or when AWS enrichment was unavailable.
	EKSNodegroups []EKSNodegroupInfo `json:"eksNodegroups,omitempty"`
	// EKSUpgradeInsights is the full inventory of AWS-native EKS Upgrade
	// Insights returned for this scan target. Passing insights are shown as
	// inventory; non-passing statuses may also create conservative
	// EKS-INSIGHT findings.
	EKSUpgradeInsights []EKSUpgradeInsightInfo `json:"eksUpgradeInsights,omitempty"`
}

// EKSAddonInfo is one installed EKS-managed add-on and its target-version
// compatibility, mirroring the same AWS-reported data ADDON-001 (see
// internal/rules/addon001.go) already uses to decide whether to raise a
// finding — this type just exposes the full inventory instead of only the
// incompatible subset.
type EKSAddonInfo struct {
	Name               string   `json:"name"`
	CurrentVersion     string   `json:"currentVersion,omitempty"`
	CompatibleVersions []string `json:"compatibleVersions,omitempty"`
	// Compatible is meaningless (always false) when VerificationUnavailable
	// is true — callers must check VerificationUnavailable first.
	Compatible bool `json:"compatible"`
	// VerificationUnavailable is true when AWS's DescribeAddonVersions call
	// failed for this specific add-on (e.g. a permissions gap) — this
	// add-on's compatibility genuinely could not be checked, which is a
	// different, more honest state than silently reporting "compatible."
	VerificationUnavailable bool `json:"verificationUnavailable,omitempty"`
}

// EKSNodegroupInfo is one EKS managed node group and its AWS-reported
// readiness context. It is inventory context first; NODE-001 remains the
// authoritative kubelet-skew check from real Kubernetes node data.
type EKSNodegroupInfo struct {
	Name                     string                    `json:"name"`
	Status                   string                    `json:"status,omitempty"`
	Version                  string                    `json:"version,omitempty"`
	ReleaseVersion           string                    `json:"releaseVersion,omitempty"`
	AMIType                  string                    `json:"amiType,omitempty"`
	CapacityType             string                    `json:"capacityType,omitempty"`
	DesiredSize              *int32                    `json:"desiredSize,omitempty"`
	MinSize                  *int32                    `json:"minSize,omitempty"`
	MaxSize                  *int32                    `json:"maxSize,omitempty"`
	MaxUnavailable           *int32                    `json:"maxUnavailable,omitempty"`
	MaxUnavailablePercentage *int32                    `json:"maxUnavailablePercentage,omitempty"`
	LaunchTemplate           bool                      `json:"launchTemplate,omitempty"`
	HealthIssues             []EKSNodegroupHealthIssue `json:"healthIssues,omitempty"`
	AutoScalingGroups        []string                  `json:"autoScalingGroups,omitempty"`
	ReadinessStatus          string                    `json:"readinessStatus"`
	Notes                    []string                  `json:"notes,omitempty"`
}

// EKSNodegroupHealthIssue is one AWS-reported managed node group health issue.
type EKSNodegroupHealthIssue struct {
	Code        string   `json:"code,omitempty"`
	Message     string   `json:"message,omitempty"`
	ResourceIDs []string `json:"resourceIds,omitempty"`
}

// EKSUpgradeInsightInfo is one AWS-native EKS Upgrade Insight returned by
// Amazon EKS. It is an additional provider signal; it does not replace
// KubePreflight's local Kubernetes checks.
type EKSUpgradeInsightInfo struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Category           string            `json:"category"`
	Status             string            `json:"status"`
	KubernetesVersion  string            `json:"kubernetesVersion,omitempty"`
	LastRefreshTime    string            `json:"lastRefreshTime,omitempty"`
	LastTransitionTime string            `json:"lastTransitionTime,omitempty"`
	Description        string            `json:"description,omitempty"`
	Recommendation     string            `json:"recommendation,omitempty"`
	AdditionalInfo     map[string]string `json:"additionalInfo,omitempty"`
	DeprecationDetails []string          `json:"deprecationDetails,omitempty"`
	AddonCompatibility []string          `json:"addonCompatibilityDetails,omitempty"`
}

// EKSClusterInfo is read-only EKS cluster metadata surfaced alongside the
// scan's findings — not a finding itself, just operator-facing context
// (which cluster, which region, which EKS platform version, whether
// extended support is enabled) shown in report.html/Console next to the
// existing current/target version chips.
type EKSClusterInfo struct {
	ClusterName string `json:"clusterName,omitempty"`
	Region      string `json:"region,omitempty"`
	// Version is the EKS-reported Kubernetes control-plane version — kept
	// distinct from Report.CurrentVersion (sourced from the Kubernetes API
	// server's own GitVersion) rather than replacing it. The two should
	// normally agree; showing both is more honest than silently picking
	// one source over the other.
	Version         string `json:"version,omitempty"`
	PlatformVersion string `json:"platformVersion,omitempty"`
	Status          string `json:"status,omitempty"`
	// SupportType is "STANDARD" or "EXTENDED" (EKS's extended support
	// program) — empty when AWS didn't report it.
	SupportType    string `json:"supportType,omitempty"`
	EndpointAccess string `json:"endpointAccess,omitempty"`
	ARN            string `json:"arn,omitempty"`
}

// NewReport builds a Report from a flat finding list, computing the summary.
func NewReport(targetVersion, clusterContext, provider string, scannedAt time.Time, fs []Finding) *Report {
	if fs == nil {
		fs = []Finding{}
	}
	r := &Report{
		SchemaVersion:  SchemaVersion,
		TargetVersion:  targetVersion,
		ClusterContext: clusterContext,
		Provider:       provider,
		ScannedAt:      scannedAt,
		Findings:       fs,
		Coverage: ScanCoverage{
			Kubernetes: PlaneCoverage{Status: CoverageComplete},
			AWS:        PlaneCoverage{Status: CoverageSkipped},
			Manifests:  PlaneCoverage{Status: CoverageSkipped},
		},
	}
	for _, f := range fs {
		if hasCrossPlaneMatch(f.Resources) && len(r.Assumptions) == 0 {
			r.Assumptions = []string{CrossPlaneManifestAssumption}
		}
		switch f.Severity {
		case SeverityBlocker:
			r.Summary.Blockers++
		case SeverityWarning:
			r.Summary.Warnings++
		case SeverityInfo:
			r.Summary.Infos++
		}
	}
	return r
}

// NormalizeKubernetesVersion extracts "major.minor" from Kubernetes version
// strings such as "v1.29.6-eks-1234567". It deliberately parses only an
// explicit control-plane/server version supplied by callers; it must not be
// fed node kubelet versions as a fallback.
func NormalizeKubernetesVersion(v string) (string, bool) {
	major, minor, err := parseMajorMinor(v)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%d.%d", major, minor), true
}

func parseMajorMinor(v string) (major, minor int, err error) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
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

// UpgradePath returns a one-minor-at-a-time display path from current to
// target, plus the operator-facing label for that gap. ok is false when the
// current control-plane version is unknown or the versions cannot form a
// same-major forward path.
func UpgradePath(currentVersion, targetVersion string) (path []string, label string, ok bool) {
	currentMajor, currentMinor, err := parseMajorMinor(currentVersion)
	if err != nil {
		return nil, "current version unknown", false
	}
	targetMajor, targetMinor, err := parseMajorMinor(targetVersion)
	if err != nil || currentMajor != targetMajor || targetMinor < currentMinor {
		return nil, "upgrade path unavailable", false
	}
	path = make([]string, 0, targetMinor-currentMinor+1)
	for minor := currentMinor; minor <= targetMinor; minor++ {
		path = append(path, fmt.Sprintf("%d.%d", currentMajor, minor))
	}
	switch targetMinor - currentMinor {
	case 0:
		label = "same-minor target"
	case 1:
		label = "one-minor upgrade"
	default:
		label = "multi-minor upgrade path"
	}
	return path, label, true
}

func hasCrossPlaneMatch(refs []ResourceReference) bool {
	liveConcepts := map[string]bool{}
	for _, ref := range refs {
		if ref.Plane != PlaneLive {
			continue
		}
		if key, ok := ref.ConceptKey(); ok {
			liveConcepts[key] = true
		}
	}
	for _, ref := range refs {
		if ref.Plane != PlaneManifest {
			continue
		}
		if key, ok := ref.ConceptKey(); ok && liveConcepts[key] {
			return true
		}
	}
	return false
}

// resultAndExitCode is the single, shared priority order for the overall
// scan outcome. Result() and ExitCode() both derive from this so they can
// never disagree: incomplete coverage outranks blockers — a scan that
// couldn't collect all its evidence is never a fully-trusted "BLOCKED" or
// "CLEAN" result, even when real blockers were found with the evidence
// that WAS collected (those blockers stay fully visible in Summary/
// Findings; only the top-level result/exit code defer to "incomplete").
func (r *Report) resultAndExitCode() (string, int) {
	switch {
	case !r.IsComplete():
		return "INCOMPLETE", 3
	case r.Summary.Blockers > 0:
		return "BLOCKED", 2
	case r.Summary.Warnings > 0:
		return "PASSED_WITH_WARNINGS", 1
	default:
		return "CLEAN", 0
	}
}

// Result classifies the overall scan outcome from the finding summary.
func (r *Report) Result() string {
	result, _ := r.resultAndExitCode()
	return result
}

// ExitCode maps the scan outcome to the CLI exit-code contract documented
// in the README: 0 clean, 1 warnings only, 2 blockers present, 3
// incomplete coverage.
func (r *Report) ExitCode() int {
	_, code := r.resultAndExitCode()
	return code
}

func (r *Report) IsComplete() bool {
	return r.Coverage.Kubernetes.Status != CoveragePartial &&
		r.Coverage.AWS.Status != CoveragePartial &&
		r.Coverage.Manifests.Status != CoveragePartial
}

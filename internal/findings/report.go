package findings

import (
	"fmt"
	"sort"
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
	// APICompatibility summarizes API-001/API-002 findings into an
	// operator-facing scorecard. It is derived from findings only; it does
	// not affect Result, exit codes, or rule severity.
	APICompatibility *APICompatibilitySummary `json:"apiCompatibility,omitempty"`
	// UpgradeReadiness generalizes the same idea across every rule family,
	// not just API compatibility. Also derived from findings only — Verdict
	// is Result() verbatim, never a second decision engine.
	UpgradeReadiness *UpgradeReadinessSummary `json:"upgradeReadiness,omitempty"`
}

// APICompatibilitySummary is an aggregate view over API compatibility
// findings. Verdict remains finding-driven (removed API blockers still
// block regardless of score); ScoreImpact is a capped readiness signal.
type APICompatibilitySummary struct {
	Status             string                 `json:"status"`
	UpgradeContinue    bool                   `json:"upgradeContinue"`
	RemovedObjects     int                    `json:"removedObjects"`
	DeprecatedObjects  int                    `json:"deprecatedObjects"`
	RemovedFamilies    []APICompatibilityItem `json:"removedFamilies,omitempty"`
	DeprecatedFamilies []APICompatibilityItem `json:"deprecatedFamilies,omitempty"`
	CriticalImpact     bool                   `json:"criticalImpact"`
	ScoreImpact        int                    `json:"scoreImpact"`
}

type APICompatibilityItem struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Count      int      `json:"count"`
	Resources  []string `json:"resources,omitempty"`
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
	// Every finding gets Priority/PriorityReason/AffectedScope/
	// CanUpgradeContinue here, once, centrally — rules themselves never
	// set these, so a rule can't forget to and every caller of NewReport
	// gets them for free. See AssignPriority (priority.go).
	prioritized := make([]Finding, len(fs))
	for i, f := range fs {
		prioritized[i] = AssignPriority(f)
	}
	fs = prioritized

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
	r.APICompatibility = BuildAPICompatibilitySummary(fs)
	// r.Result() here reads the placeholder, always-complete Coverage set
	// above -- correct for every caller that never calls SetCoverage
	// (the common case: a fully successful scan, and the large majority of
	// tests that don't exercise partial coverage at all). Callers that do
	// discover partial/incomplete coverage after this point (scan.go,
	// plan.go, once the real collector results are known) must use
	// SetCoverage, not direct field assignment, or UpgradeReadiness stays
	// silently wrong -- see SetCoverage's own comment.
	r.UpgradeReadiness = BuildUpgradeReadinessSummary(fs, r.Result())
	return r
}

// SetCoverage replaces the report's ScanCoverage and recomputes
// UpgradeReadiness to match. NewReport itself always builds UpgradeReadiness
// against a placeholder, fully-complete Coverage, since a report's real
// Coverage is only known once every collector has finished -- after
// NewReport already had to run (Coverage needs the same collector snapshots
// AssignPriority/Summary/APICompatibility above are built from). Setting
// r.Coverage directly instead of through this method leaves
// r.UpgradeReadiness.Verdict/ReadinessScore/UpgradeContinue stuck at
// whatever they'd be for a fully complete scan, silently disagreeing with
// r.Result()/r.ExitCode() the moment coverage actually turns out partial or
// incomplete (confirmed: a --provider=eks scan with missing IAM permissions
// exits 3/INCOMPLETE but would report UpgradeReadiness.Verdict "CLEAN").
func (r *Report) SetCoverage(c ScanCoverage) {
	r.Coverage = c
	r.UpgradeReadiness = BuildUpgradeReadinessSummary(r.Findings, r.Result())
}

func BuildAPICompatibilitySummary(fs []Finding) *APICompatibilitySummary {
	summary := &APICompatibilitySummary{
		Status:          "Passed",
		UpgradeContinue: true,
	}
	removedFamilies := map[string]*APICompatibilityItem{}
	deprecatedFamilies := map[string]*APICompatibilityItem{}

	for _, f := range fs {
		if f.RuleID != "API-001" && f.RuleID != "API-002" {
			continue
		}
		family := apiCompatibilityFamily(f)
		if family.APIVersion == "" && family.Kind == "" {
			continue
		}
		if f.RuleID == "API-001" && f.Severity == SeverityBlocker {
			summary.RemovedObjects++
			addAPICompatibilityFamily(removedFamilies, family, f)
			summary.UpgradeContinue = false
			summary.Status = "Failed"
			if apiCompatibilityCriticalImpact(f) {
				summary.CriticalImpact = true
			}
			continue
		}
		if f.RuleID == "API-002" || f.Severity == SeverityWarning {
			summary.DeprecatedObjects++
			addAPICompatibilityFamily(deprecatedFamilies, family, f)
			if summary.Status == "Passed" {
				summary.Status = "Warning"
			}
		}
	}

	summary.RemovedFamilies = sortedAPICompatibilityFamilies(removedFamilies)
	summary.DeprecatedFamilies = sortedAPICompatibilityFamilies(deprecatedFamilies)
	summary.ScoreImpact = apiCompatibilityScoreImpact(summary)
	if summary.RemovedObjects == 0 && summary.DeprecatedObjects == 0 {
		return summary
	}
	return summary
}

func apiCompatibilityFamily(f Finding) APICompatibilityItem {
	item := APICompatibilityItem{}
	for _, evidence := range f.Evidence {
		if strings.HasPrefix(evidence, "apiVersion: ") {
			item.APIVersion = strings.TrimSpace(strings.TrimPrefix(evidence, "apiVersion: "))
			break
		}
	}
	if len(f.Resources) > 0 {
		item.Kind = f.Resources[0].Kind
	}
	return item
}

func addAPICompatibilityFamily(families map[string]*APICompatibilityItem, family APICompatibilityItem, f Finding) {
	key := family.APIVersion + "\x00" + family.Kind
	item, ok := families[key]
	if !ok {
		item = &APICompatibilityItem{APIVersion: family.APIVersion, Kind: family.Kind}
		families[key] = item
	}
	item.Count++
	for _, ref := range f.Resources {
		label := ref.Kind + "/" + ref.Name
		if ref.Namespace != "" {
			label = ref.Kind + "/" + ref.Namespace + "/" + ref.Name
		}
		item.Resources = appendUniqueStrings(item.Resources, label)
	}
}

func sortedAPICompatibilityFamilies(families map[string]*APICompatibilityItem) []APICompatibilityItem {
	out := make([]APICompatibilityItem, 0, len(families))
	for _, item := range families {
		sort.Strings(item.Resources)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].APIVersion != out[j].APIVersion {
			return out[i].APIVersion < out[j].APIVersion
		}
		return out[i].Kind < out[j].Kind
	})
	return out
}

func apiCompatibilityCriticalImpact(f Finding) bool {
	if f.GlobalBlocker || f.CriticalInfra {
		return true
	}
	for _, ref := range f.Resources {
		if ref.Scope == ScopeCluster {
			return true
		}
	}
	return false
}

func apiCompatibilityScoreImpact(summary *APICompatibilitySummary) int {
	impact := 0
	if len(summary.RemovedFamilies) > 0 {
		impact -= 25
		if len(summary.RemovedFamilies) > 1 {
			impact -= (len(summary.RemovedFamilies) - 1) * 10
		}
	}
	if summary.CriticalImpact {
		impact -= 15
	}
	impact -= len(summary.DeprecatedFamilies) * 5
	if impact < -60 {
		return -60
	}
	return impact
}

func appendUniqueStrings(dst []string, values ...string) []string {
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

// UpgradeReadinessSummary generalizes APICompatibilitySummary's idea across
// every registered rule family, not just API compatibility — a
// per-category Passed/Warning/Failed breakdown plus one consolidated
// verdict and score. Like APICompatibility, this is derived from findings
// only: Verdict is Report.Result() verbatim (never recomputed here), so the
// scorecard can never disagree with the real exit-code-driving result.
// ReadinessScore is a separate, softer 0-100 signal — useful for trending
// across scans — deliberately kept apart from Verdict/UpgradeContinue,
// which stay hard finding-driven facts.
type UpgradeReadinessSummary struct {
	Verdict         string                     `json:"verdict"`
	UpgradeContinue bool                       `json:"upgradeContinue"`
	ReadinessScore  int                        `json:"readinessScore"`
	Categories      []UpgradeReadinessCategory `json:"categories"`
}

type UpgradeReadinessCategory struct {
	Name         string   `json:"name"`
	Status       string   `json:"status"` // Passed / Warning / Failed
	BlockerCount int      `json:"blockerCount"`
	WarningCount int      `json:"warningCount"`
	RuleIDs      []string `json:"ruleIds"`
}

// categoryOrder is both the display order and the authoritative category
// list — categoryByRuleID's values must all appear here, checked by
// TestUpgradeReadinessCategories_RuleIDsAreConsistent.
var categoryOrder = []string{
	"API Compatibility",
	"Extension APIs",
	"Admission Webhooks",
	"Disruption Safety",
	"Node Readiness",
	"Add-ons",
	"CoreDNS",
	"Workload Health",
	"EKS Upgrade Insights",
}

// categoryByRuleID maps every registered rule ID to one scorecard category.
// internal/findings/upgrade_readiness_registry_test.go (package
// findings_test, avoiding an import cycle) asserts every rule ID in
// rules.NewDefaultRegistry().RuleIDs() has an entry here, so a newly
// registered rule fails CI instead of silently landing in no category.
var categoryByRuleID = map[string]string{
	"API-001": "API Compatibility",
	"API-002": "API Compatibility",

	"CRD-001":        "Extension APIs",
	"CRD-002":        "Extension APIs",
	"APISERVICE-001": "Extension APIs",

	"WH-001": "Admission Webhooks",
	"WH-002": "Admission Webhooks",
	"WH-004": "Admission Webhooks",
	"WH-005": "Admission Webhooks",

	"PDB-001": "Disruption Safety",
	"PDB-002": "Disruption Safety",

	"NODE-001":   "Node Readiness",
	"NODE-002":   "Node Readiness",
	"NODE-003":   "Node Readiness",
	"NET-002":    "Node Readiness",
	"EKS-NG-001": "Node Readiness",
	"EKS-NG-002": "Node Readiness",
	"EKS-NG-003": "Node Readiness",
	"EKS-NG-004": "Node Readiness",

	"ADDON-001": "Add-ons",
	"ADDON-002": "Add-ons",

	"COREDNS-001": "CoreDNS",

	"WORKLOAD-001": "Workload Health",

	"EKS-INSIGHT-001": "EKS Upgrade Insights",
	"EKS-INSIGHT-002": "EKS Upgrade Insights",
	"EKS-INSIGHT-003": "EKS Upgrade Insights",
}

// HasExplicitCategoryMapping reports whether ruleID has its own entry in
// categoryByRuleID. Exported only for the registry-coverage test — mirrors
// HasExplicitPriorityMapping (priority.go)'s exact purpose and pattern.
func HasExplicitCategoryMapping(ruleID string) bool {
	_, ok := categoryByRuleID[ruleID]
	return ok
}

// BuildUpgradeReadinessSummary aggregates fs into the consolidated
// scorecard. verdict is Report.Result(), passed in rather than recomputed
// so this function has no dependency on Report's own field layout.
func BuildUpgradeReadinessSummary(fs []Finding, verdict string) *UpgradeReadinessSummary {
	byCategory := make(map[string]*UpgradeReadinessCategory, len(categoryOrder))
	for _, name := range categoryOrder {
		byCategory[name] = &UpgradeReadinessCategory{Name: name, Status: "Passed"}
	}

	for _, f := range fs {
		name, ok := categoryByRuleID[f.RuleID]
		if !ok {
			continue
		}
		cat := byCategory[name]
		cat.RuleIDs = appendUniqueStrings(cat.RuleIDs, f.RuleID)
		switch f.Severity {
		case SeverityBlocker:
			cat.BlockerCount++
			cat.Status = "Failed"
		case SeverityWarning:
			cat.WarningCount++
			if cat.Status != "Failed" {
				cat.Status = "Warning"
			}
		}
	}

	categories := make([]UpgradeReadinessCategory, 0, len(categoryOrder))
	score := 100
	for _, name := range categoryOrder {
		cat := *byCategory[name]
		sort.Strings(cat.RuleIDs)
		categories = append(categories, cat)
		score -= upgradeReadinessCategoryPenalty(cat)
	}
	if score < 0 {
		score = 0
	}

	return &UpgradeReadinessSummary{
		Verdict: verdict,
		// verdict != "INCOMPLETE" matters even though upgradeReadinessAnyBlocker
		// already covers the BLOCKED case: a scan can be INCOMPLETE (partial
		// collector coverage) with zero blockers found *so far*, e.g.
		// --provider=eks with missing IAM permissions. Zero blockers out of
		// an incomplete evidence set must never read as "safe to continue" —
		// see SetCoverage's comment for how this was found.
		UpgradeContinue: verdict != "INCOMPLETE" && !upgradeReadinessAnyBlocker(categories),
		ReadinessScore:  score,
		Categories:      categories,
	}
}

// upgradeReadinessCategoryPenalty mirrors apiCompatibilityScoreImpact's
// shape: a base penalty for the category's status plus an incremental
// penalty for additional findings beyond the first, capped so one very
// noisy category can't single-handedly zero the score.
func upgradeReadinessCategoryPenalty(cat UpgradeReadinessCategory) int {
	if cat.Status == "Failed" {
		penalty := 15 + 3*(cat.BlockerCount-1)
		if penalty > 25 {
			return 25
		}
		return penalty
	}
	if cat.Status == "Warning" {
		penalty := 5 + (cat.WarningCount - 1)
		if penalty > 10 {
			return 10
		}
		return penalty
	}
	return 0
}

func upgradeReadinessAnyBlocker(categories []UpgradeReadinessCategory) bool {
	for _, cat := range categories {
		if cat.BlockerCount > 0 {
			return true
		}
	}
	return false
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

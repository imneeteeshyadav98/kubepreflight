package rollback

import (
	"fmt"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

func ApplyOperationalReadiness(assessment Assessment, report *findings.Report) Assessment {
	if report == nil {
		check := Check{
			ID:          "operational-evidence",
			Title:       "KubePreflight operational evidence is available",
			Status:      CheckUnknown,
			ReasonCodes: []ReasonCode{ReasonObservabilityEvidenceMissing},
		}
		assessment.Checks = append(assessment.Checks, check)
		assessment.Readiness = combineReadiness(assessment.Readiness, readinessFromChecks([]Check{check}))
		assessment.Evidence.Complete = false
		return assessment
	}

	identity := validateClusterEvidenceIdentity(report, assessment.Cluster)

	checks := []Check{
		nodegroupRollbackCheck(assessment, report, identity),
		selfManagedNodeEvidenceCheck(report),
		fargateEvidenceCheck(report),
		managedAddonCheck(report, identity),
		selfManagedAddonCheck(report, identity),
		workloadHealthCheck(report, identity),
		disruptionCheck(assessment, report, identity),
		reverseCompatibilityCheck(assessment, report, identity),
		coverageCheck(report),
	}
	assessment.Checks = append(assessment.Checks, checks...)
	assessment.Readiness = combineReadiness(assessment.Readiness, readinessFromChecks(checks))
	if hasUnknownCheck(checks) || assessment.Readiness.Status == ReadinessInsufficientEvidence {
		assessment.Evidence.Complete = false
	}
	return assessment
}

func nodegroupRollbackCheck(assessment Assessment, report *findings.Report, identity clusterEvidenceIdentity) Check {
	check := Check{
		ID:     "managed-nodegroups",
		Title:  "Managed node groups are compatible with rollback target",
		Status: CheckPass,
	}
	if identity.blocksClusterSpecificEvidence() {
		return applyIdentityGate(check, identity)
	}
	if len(report.EKSNodegroups) == 0 {
		check.Evidence = []string{"EKS managed node group inventory: none reported"}
		return check
	}
	for _, ng := range report.EKSNodegroups {
		if ng.Version == "" {
			check.Status = maxCheckStatus(check.Status, CheckUnknown)
			check.ReasonCodes = appendUniqueReason(check.ReasonCodes, ReasonSelfManagedNodeEvidenceUnavailable)
			check.Evidence = append(check.Evidence, fmt.Sprintf("nodegroup %s version: unknown", ng.Name))
			continue
		}
		check.Evidence = append(check.Evidence, fmt.Sprintf("nodegroup %s version: %s status: %s", ng.Name, ng.Version, emptyAsUnknown(ng.Status)))
		if assessment.Cluster.CurrentVersion != "" && ng.Version == assessment.Cluster.CurrentVersion && assessment.Cluster.RollbackTargetVersion != "" && ng.Version != assessment.Cluster.RollbackTargetVersion {
			check.Status = maxCheckStatus(check.Status, CheckWarning)
			check.ReasonCodes = appendUniqueReason(check.ReasonCodes, ReasonManagedNodegroupRollbackRequired)
		}
		if len(ng.HealthIssues) > 0 {
			check.Status = maxCheckStatus(check.Status, CheckWarning)
			check.ReasonCodes = appendUniqueReason(check.ReasonCodes, ReasonManagedNodegroupRollbackRequired)
		}
	}
	return check
}

func selfManagedNodeEvidenceCheck(report *findings.Report) Check {
	check := Check{
		ID:     "self-managed-node-evidence",
		Title:  "Self-managed and hybrid node evidence is available",
		Status: CheckPass,
		Evidence: []string{
			"kubernetes coverage: " + string(report.Coverage.Kubernetes.Status),
		},
	}
	if report.Coverage.Kubernetes.Status != findings.CoverageComplete {
		check.Status = CheckUnknown
		check.ReasonCodes = []ReasonCode{ReasonSelfManagedNodeEvidenceUnavailable}
	}
	return check
}

func fargateEvidenceCheck(report *findings.Report) Check {
	check := Check{
		ID:     "fargate-evidence",
		Title:  "Fargate rollback implications are identified",
		Status: CheckPass,
	}
	if report.Provider != "eks" {
		check.Evidence = []string{"provider is not EKS"}
		return check
	}
	if report.Coverage.AWS.Status != findings.CoverageComplete {
		check.Status = CheckUnknown
		check.ReasonCodes = []ReasonCode{ReasonFargateEvidenceUnavailable}
		check.Evidence = []string{"AWS coverage: " + string(report.Coverage.AWS.Status)}
		return check
	}
	if findingCount(report.Findings, "FARGATE-") > 0 {
		check.Status = CheckWarning
		check.ReasonCodes = []ReasonCode{ReasonFargatePodRecreationRisk}
		check.Evidence = []string{"Fargate-related findings present"}
		return check
	}
	check.Evidence = []string{"No Fargate-specific findings present in current evidence"}
	return check
}

func managedAddonCheck(report *findings.Report, identity clusterEvidenceIdentity) Check {
	check := Check{
		ID:     "managed-addons",
		Title:  "EKS managed add-ons are compatible with rollback target",
		Status: CheckPass,
	}
	if identity.blocksClusterSpecificEvidence() {
		return applyIdentityGate(check, identity)
	}
	for _, addon := range report.EKSAddons {
		check.Evidence = append(check.Evidence, fmt.Sprintf("addon %s version: %s compatible: %t verificationUnavailable: %t",
			addon.Name, emptyAsUnknown(addon.CurrentVersion), addon.Compatible, addon.VerificationUnavailable))
		if addon.VerificationUnavailable {
			check.Status = maxCheckStatus(check.Status, CheckUnknown)
			check.ReasonCodes = appendUniqueReason(check.ReasonCodes, ReasonManagedAddonCompatibilityUnknown)
			continue
		}
		if !addon.Compatible {
			check.Status = maxCheckStatus(check.Status, CheckWarning)
			check.ReasonCodes = appendUniqueReason(check.ReasonCodes, ReasonManagedAddonRollbackRequired)
		}
	}
	applyFindingSignals(&check, report.Findings, []string{"ADDON-001"}, ReasonManagedAddonRollbackRequired)
	applyFindingSignals(&check, report.Findings, []string{"ADDON-002"}, ReasonManagedAddonCompatibilityUnknown)
	if len(check.Evidence) == 0 {
		check.Evidence = []string{"No managed add-on compatibility findings present"}
	}
	return check
}

func selfManagedAddonCheck(report *findings.Report, identity clusterEvidenceIdentity) Check {
	check := Check{
		ID:     "self-managed-addons",
		Title:  "Self-managed add-on rollback compatibility is verified",
		Status: CheckPass,
	}
	if identity.blocksClusterSpecificEvidence() {
		return applyIdentityGate(check, identity)
	}
	applyFindingSignals(&check, report.Findings, []string{"ADDON-002"}, ReasonSelfManagedAddonCompatibilityUnknown)
	if check.Status == CheckPass {
		check.Evidence = []string{"No self-managed add-on compatibility warnings present"}
	}
	return check
}

func workloadHealthCheck(report *findings.Report, identity clusterEvidenceIdentity) Check {
	check := Check{
		ID:     "workload-health",
		Title:  "Workloads are healthy before rollback",
		Status: CheckPass,
	}
	if identity.blocksClusterSpecificEvidence() {
		return applyIdentityGate(check, identity)
	}
	applyFindingSignals(&check, report.Findings, []string{"WORKLOAD-001", "DRAIN-005"}, ReasonUnhealthyWorkloadsPresent)
	if check.Status == CheckPass {
		check.Evidence = []string{"No unhealthy workload findings present"}
	}
	return check
}

func disruptionCheck(assessment Assessment, report *findings.Report, identity clusterEvidenceIdentity) Check {
	check := Check{
		ID:     "disruption-readiness",
		Title:  "PDB and drain constraints do not block rollback preparation",
		Status: CheckPass,
	}
	if identity.blocksClusterSpecificEvidence() {
		return applyIdentityGate(check, identity)
	}
	evidence := rollbackOperationEvidenceFromAssessment(assessment, report)
	for _, f := range report.Findings {
		if !isDisruptionFinding(f.RuleID) {
			continue
		}
		routeDisruptionFinding(&check, f, evidence)
	}
	if check.Status == CheckPass {
		check.Evidence = []string{"No PDB or drain readiness findings present"}
	}
	return check
}

type rollbackOperationEvidence struct {
	nodeDrainRequired      bool
	workerReplacement      bool
	podEvictionRequired    bool
	workloadRestartNeeded  bool
	activationEvidenceSeen bool
}

func rollbackOperationEvidenceFromAssessment(_ Assessment, _ *findings.Report) rollbackOperationEvidence {
	return rollbackOperationEvidence{}
}

func routeDisruptionFinding(check *Check, f findings.Finding, evidence rollbackOperationEvidence) {
	if f.RuleID == "DRAIN-005" {
		return
	}
	check.Evidence = append(check.Evidence, fmt.Sprintf("%s %s: %s", f.RuleID, f.Severity, f.Message))
	check.ReasonCodes = appendUniqueReason(check.ReasonCodes, ReasonPDBDisruptionConstraints)
	status := CheckWarning
	if f.Severity == findings.SeverityBlocker && disruptionActivationConfirmed(f, evidence) {
		status = CheckFail
	}
	check.Status = maxCheckStatus(check.Status, status)
}

func isDisruptionFinding(ruleID string) bool {
	return strings.HasPrefix(ruleID, "PDB-") || strings.HasPrefix(ruleID, "DRAIN-")
}

func disruptionActivationConfirmed(f findings.Finding, evidence rollbackOperationEvidence) bool {
	if !evidence.activationEvidenceSeen || !rollbackBlockingDisruptionRule(f.RuleID) {
		return false
	}
	if len(f.ImpactScopes) == 0 {
		return evidence.nodeDrainRequired || evidence.workerReplacement || evidence.podEvictionRequired || evidence.workloadRestartNeeded
	}
	for _, scope := range f.ImpactScopes {
		switch scope {
		case findings.ImpactScopeNodeDrain:
			if evidence.nodeDrainRequired || evidence.podEvictionRequired || evidence.workerReplacement {
				return true
			}
		case findings.ImpactScopeWorkerRollout:
			if evidence.workerReplacement || evidence.nodeDrainRequired {
				return true
			}
		case findings.ImpactScopeWorkloadRestart:
			if evidence.workloadRestartNeeded || evidence.podEvictionRequired {
				return true
			}
		}
	}
	return false
}

func rollbackBlockingDisruptionRule(ruleID string) bool {
	switch ruleID {
	case "PDB-001", "PDB-002", "DRAIN-002":
		return true
	default:
		return false
	}
}

// reverseCompatibilityCheck combines two independently-gated evidence
// families into one check:
//
//   - API-001/API-002 findings are gated only by validateAPIEvidenceTarget
//     (findings.json TargetVersion vs. Cluster.RollbackTargetVersion) --
//     deliberately not by cluster identity. Distinguishing which individual
//     API-001/API-002 findings are live-cluster-specific vs.
//     manifest-derived would require per-finding Plane inspection this PR
//     does not add (see validateClusterEvidenceIdentity's doc comment and
//     docs/rollback-readiness.md's "API evidence target validation"
//     section); this stays exactly as PR #207 left it.
//   - CRD-*/WH-* findings describe current live cluster state (see
//     internal/rules/crd001.go, crd002.go: both require sc.K8s and produce
//     nothing for a manifest-only scan) and are gated by cluster identity
//     like every other check in this file.
//
// Both paths write into the same Check via maxCheckStatus/appendUniqueReason
// so a mismatched cluster identity can never be masked by a validated API
// target, and a validated API target can never suppress an identity
// mismatch reason.
func reverseCompatibilityCheck(assessment Assessment, report *findings.Report, identity clusterEvidenceIdentity) Check {
	check := Check{
		ID:     "reverse-compatibility",
		Title:  "API, CRD, and webhook state is compatible with rollback target",
		Status: CheckPass,
	}
	applyAPIEvidenceFindings(&check, assessment, report)
	if identity.blocksClusterSpecificEvidence() {
		check.Status = maxCheckStatus(check.Status, CheckUnknown)
		check.ReasonCodes = appendUniqueReason(check.ReasonCodes, identity.reason)
		check.Evidence = append(check.Evidence, identity.evidence...)
	} else {
		applyFindingSignals(&check, report.Findings, []string{"CRD-", "WH-"}, ReasonCRDWebhookControllerRisk)
	}
	if check.Status == CheckPass {
		check.Evidence = []string{"No API, CRD, or webhook rollback compatibility findings present"}
	}
	return check
}

// applyAPIEvidenceFindings routes API-001/API-002 findings into check only
// when they are valid rollback evidence for the actual rollback target.
// API-001 and API-002 are forward-target-gated rules (see
// targetReachesRemoval/targetBeforeRemoval in internal/rules) — their raw
// severity was computed against report.TargetVersion, which is not
// necessarily assessment.Cluster.RollbackTargetVersion (a findings.json
// generated for a future upgrade may be supplied to a rollback assessment).
// When the two targets don't provably match, these findings' raw severity
// is not confirmed rollback evidence: this deliberately does not recompute
// API compatibility against the rollback target, it only decides whether
// the supplied evidence's own stated target is the right one to trust.
func applyAPIEvidenceFindings(check *Check, assessment Assessment, report *findings.Report) {
	apiFindings := findingsWithPrefixes(report.Findings, []string{"API-001", "API-002"})
	if len(apiFindings) == 0 {
		// Nothing to validate provenance for -- do not manufacture a
		// mismatch reason when there is no API-001/API-002 evidence.
		return
	}
	status, reason, evidence := validateAPIEvidenceTarget(report.TargetVersion, assessment.Cluster.RollbackTargetVersion)
	if status == apiEvidenceTargetValidated {
		applyFindingSignals(check, apiFindings, []string{"API-001", "API-002"}, ReasonNewVersionAPIAdoptionRisk)
		return
	}
	check.Status = maxCheckStatus(check.Status, CheckUnknown)
	check.ReasonCodes = appendUniqueReason(check.ReasonCodes, reason)
	check.Evidence = append(check.Evidence, evidence...)
}

// apiEvidenceTargetStatus is the outcome of validateAPIEvidenceTarget.
type apiEvidenceTargetStatus int

const (
	// apiEvidenceTargetValidated means both versions are known and
	// normalize to the same Kubernetes minor version -- the supplied
	// API-001/API-002 findings are valid rollback evidence.
	apiEvidenceTargetValidated apiEvidenceTargetStatus = iota
	// apiEvidenceTargetMismatch means both versions are known but
	// normalize to different Kubernetes minor versions.
	apiEvidenceTargetMismatch
	// apiEvidenceTargetUnknown means one or both versions are missing or
	// unparseable, so provenance cannot be confirmed either way.
	apiEvidenceTargetUnknown
)

// validateAPIEvidenceTarget compares a findings report's target version
// against the actual rollback target at Kubernetes minor-version
// granularity, using the same normalization findings.NormalizeKubernetesVersion
// already applies elsewhere (so "v1.34", "1.34", and "1.34.2" are all the
// same target, but "1.34" and "1.36" are not). It never recalculates API
// compatibility itself -- it only decides whether previously computed
// API-001/API-002 findings carry the right target provenance to be trusted
// as rollback evidence for this specific rollback target.
func validateAPIEvidenceTarget(findingsTargetVersion, rollbackTargetVersion string) (apiEvidenceTargetStatus, ReasonCode, []string) {
	findingsTargetVersion = strings.TrimSpace(findingsTargetVersion)
	rollbackTargetVersion = strings.TrimSpace(rollbackTargetVersion)
	if findingsTargetVersion == "" || rollbackTargetVersion == "" {
		return apiEvidenceTargetUnknown, ReasonRollbackEvidenceTargetUnknown, []string{
			fmt.Sprintf("API compatibility evidence target provenance unknown: findings target=%s rollback target=%s",
				emptyAsUnknown(findingsTargetVersion), emptyAsUnknown(rollbackTargetVersion)),
		}
	}
	findingsMinor, findingsOK := findings.NormalizeKubernetesVersion(findingsTargetVersion)
	rollbackMinor, rollbackOK := findings.NormalizeKubernetesVersion(rollbackTargetVersion)
	if !findingsOK || !rollbackOK {
		return apiEvidenceTargetUnknown, ReasonRollbackEvidenceTargetUnknown, []string{
			fmt.Sprintf("API compatibility evidence target provenance unknown: findings target=%s rollback target=%s (unparseable version)",
				findingsTargetVersion, rollbackTargetVersion),
		}
	}
	if findingsMinor != rollbackMinor {
		return apiEvidenceTargetMismatch, ReasonRollbackEvidenceTargetMismatch, []string{
			fmt.Sprintf("API compatibility evidence target mismatch: findings target=%s rollback target=%s",
				findingsTargetVersion, rollbackTargetVersion),
		}
	}
	return apiEvidenceTargetValidated, "", nil
}

// clusterEvidenceIdentityStatus is the outcome of
// validateClusterEvidenceIdentity, mirroring apiEvidenceTargetStatus's
// shape from PR #207.
type clusterEvidenceIdentityStatus int

const (
	// clusterEvidenceIdentityMatch means the findings report's live EKS
	// cluster name and region (trimmed) are both known and match the
	// rollback assessment's cluster name and region exactly -- supplied
	// cluster-specific findings are valid evidence for this cluster.
	clusterEvidenceIdentityMatch clusterEvidenceIdentityStatus = iota
	// clusterEvidenceIdentityMismatch means both sides have a known cluster
	// name and region, but the name and/or region differs -- the findings
	// report almost certainly describes a different cluster.
	clusterEvidenceIdentityMismatch
	// clusterEvidenceIdentityUnknown means the report is expected to carry
	// live cluster identity (it is not a manifest-only report) but the
	// cluster name and/or region is missing or unparseable on either side,
	// so same-cluster provenance can't be confirmed either way.
	clusterEvidenceIdentityUnknown
	// clusterEvidenceIdentityNotApplicable means the findings report has no
	// live cluster evidence at all (a manifest-only scan: no EKS enrichment
	// and no kubeconfig context was ever loaded) -- there is no wrong-cluster
	// claim to evaluate, only an absence of live-cluster-specific evidence.
	clusterEvidenceIdentityNotApplicable
)

// clusterEvidenceIdentity is the once-computed result of
// validateClusterEvidenceIdentity, threaded through every affected check so
// identity comparison is never reimplemented per check (see
// ApplyOperationalReadiness).
type clusterEvidenceIdentity struct {
	status   clusterEvidenceIdentityStatus
	reason   ReasonCode
	evidence []string
}

// blocksClusterSpecificEvidence reports whether a check that depends on
// confirmed same-cluster live evidence must route to CheckUnknown instead
// of consuming the report's cluster-specific findings/inventory. A
// not-applicable (manifest-only) report does not block: there is no live
// cluster identity to have gotten wrong, and (per
// validateClusterEvidenceIdentity's doc comment) the check families this
// gates have nothing live-plane to consume from such a report anyway.
func (id clusterEvidenceIdentity) blocksClusterSpecificEvidence() bool {
	return id.status == clusterEvidenceIdentityMismatch || id.status == clusterEvidenceIdentityUnknown
}

// applyIdentityGate replaces check's outcome with a confirmed-insufficient-
// evidence result carrying identity's reason code and sanitized evidence
// text, instead of letting the check consume unconfirmed cluster-specific
// findings/inventory. Used uniformly by every check family
// validateClusterEvidenceIdentity gates -- see
// clusterEvidenceIdentity.blocksClusterSpecificEvidence.
func applyIdentityGate(check Check, identity clusterEvidenceIdentity) Check {
	check.Status = CheckUnknown
	check.ReasonCodes = appendUniqueReason(check.ReasonCodes, identity.reason)
	check.Evidence = append(check.Evidence, identity.evidence...)
	return check
}

// validateClusterEvidenceIdentity compares the supplied findings report's
// live EKS cluster identity against the rollback assessment's own live
// cluster identity (assessment.Cluster.Name/Region, populated from a live
// DescribeCluster call in internal/rollback/eks/eligibility.go), so
// cluster-specific operational checks -- node groups, managed/self-managed
// add-ons, workload health, PDB/drain disruption, and CRD/webhook current
// state -- never silently consume evidence collected against a different
// cluster.
//
// Only cluster name and region are compared -- the strongest identity this
// PR uses. report.EKSCluster.ARN is destroyed to a fixed
// "[redacted-arn]" placeholder by --redact-sensitive-identifiers (see
// redact.Report/redact.RollbackAssessment), so it cannot be used as
// identity without breaking matching for redacted reports; no AWS account
// ID, API-server endpoint, or Kubernetes cluster UID is collected today.
// Cluster name and region are explicitly NOT redacted (see
// redact.RollbackAssessment's doc comment), so this comparison behaves
// identically for redacted and unredacted reports. See
// docs/rollback-readiness.md's "Findings cluster identity validation"
// section for what's intentionally out of scope.
//
// Comparison is exact after TrimSpace on both sides -- no case-folding.
// AWS cluster names and regions are case-sensitive by convention, unlike
// findings.NormalizeKubernetesVersion's version-string normalization, which
// this function deliberately does not mirror.
//
// report.ClusterContext (a local kubeconfig label, not a validated cluster
// identity -- see findings.Report's doc comment) is used only to decide
// whether the report claims to be live-cluster evidence at all; it is never
// compared as an identity value itself.
//
// A findings report with neither EKSCluster nor ClusterContext populated is
// a manifest-only scan (see internal/cli/scan.go: --manifests-only never
// loads a kubeconfig and never attempts AWS enrichment) and is treated as
// clusterEvidenceIdentityNotApplicable, not a mismatch: there is no
// wrong-cluster claim to evaluate. The check families this result gates
// each require live cluster evidence to produce anything in the first place
// (internal/rules/crd001.go, crd002.go, pdb001.go, pdb002.go, workload001.go
// all require sc.K8s; report.EKSNodegroups/EKSAddons are only ever
// populated from a live AWS DescribeCluster-adjacent call), so a
// manifest-only report has nothing live-plane for those checks to
// mistakenly confirm.
func validateClusterEvidenceIdentity(report *findings.Report, cluster Cluster) clusterEvidenceIdentity {
	if report.EKSCluster == nil && strings.TrimSpace(report.ClusterContext) == "" {
		return clusterEvidenceIdentity{status: clusterEvidenceIdentityNotApplicable}
	}

	var findingsName, findingsRegion string
	if report.EKSCluster != nil {
		findingsName = strings.TrimSpace(report.EKSCluster.ClusterName)
		findingsRegion = strings.TrimSpace(report.EKSCluster.Region)
	}
	assessedName := strings.TrimSpace(cluster.Name)
	assessedRegion := strings.TrimSpace(cluster.Region)

	if findingsName == "" || findingsRegion == "" || assessedName == "" || assessedRegion == "" {
		return clusterEvidenceIdentity{
			status: clusterEvidenceIdentityUnknown,
			reason: ReasonRollbackEvidenceClusterUnknown,
			evidence: []string{fmt.Sprintf(
				"cluster identity provenance unknown: findings cluster=%s region=%s vs assessed cluster=%s region=%s",
				emptyAsUnknown(findingsName), emptyAsUnknown(findingsRegion),
				emptyAsUnknown(assessedName), emptyAsUnknown(assessedRegion)),
			},
		}
	}

	if findingsName != assessedName || findingsRegion != assessedRegion {
		return clusterEvidenceIdentity{
			status: clusterEvidenceIdentityMismatch,
			reason: ReasonRollbackEvidenceClusterMismatch,
			evidence: []string{fmt.Sprintf(
				"cluster identity mismatch: findings cluster=%s region=%s vs assessed cluster=%s region=%s",
				findingsName, findingsRegion, assessedName, assessedRegion),
			},
		}
	}

	return clusterEvidenceIdentity{status: clusterEvidenceIdentityMatch}
}

// findingsWithPrefixes filters fs down to findings whose RuleID matches one
// of prefixes, using the same matching rule ruleMatches/applyFindingSignals
// already use elsewhere in this file.
func findingsWithPrefixes(fs []findings.Finding, prefixes []string) []findings.Finding {
	var out []findings.Finding
	for _, f := range fs {
		if ruleMatches(f.RuleID, prefixes) {
			out = append(out, f)
		}
	}
	return out
}

func coverageCheck(report *findings.Report) Check {
	check := Check{
		ID:     "evidence-coverage",
		Title:  "Operational evidence coverage is complete",
		Status: CheckPass,
		Evidence: []string{
			"kubernetes coverage: " + string(report.Coverage.Kubernetes.Status),
			"aws coverage: " + string(report.Coverage.AWS.Status),
			"manifest coverage: " + string(report.Coverage.Manifests.Status),
		},
	}
	if report.Coverage.Kubernetes.Status == "" {
		check.Evidence[0] = "kubernetes coverage: unknown"
	}
	if report.Coverage.Kubernetes.Status == findings.CoveragePartial ||
		report.Coverage.AWS.Status == findings.CoveragePartial ||
		report.Coverage.Manifests.Status == findings.CoveragePartial {
		check.Status = CheckUnknown
		check.ReasonCodes = []ReasonCode{ReasonObservabilityEvidenceMissing}
	}
	return check
}

func applyFindingSignals(check *Check, fs []findings.Finding, prefixes []string, reason ReasonCode) {
	for _, f := range fs {
		if !ruleMatches(f.RuleID, prefixes) {
			continue
		}
		check.Evidence = append(check.Evidence, fmt.Sprintf("%s %s: %s", f.RuleID, f.Severity, f.Message))
		check.ReasonCodes = appendUniqueReason(check.ReasonCodes, reason)
		switch f.Severity {
		case findings.SeverityBlocker:
			check.Status = maxCheckStatus(check.Status, CheckFail)
		case findings.SeverityWarning:
			check.Status = maxCheckStatus(check.Status, CheckWarning)
		default:
			check.Status = maxCheckStatus(check.Status, CheckWarning)
		}
	}
}

func ruleMatches(ruleID string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasSuffix(prefix, "-") {
			if strings.HasPrefix(ruleID, prefix) {
				return true
			}
			continue
		}
		if ruleID == prefix {
			return true
		}
	}
	return false
}

func readinessFromChecks(checks []Check) Readiness {
	var out Readiness
	for _, check := range checks {
		switch check.Status {
		case CheckFail:
			out.Blockers++
		case CheckWarning:
			out.Warnings++
		case CheckUnknown:
			out.Unknowns++
		}
	}
	switch {
	case out.Blockers > 0:
		out.Status = ReadinessBlocked
	case out.Unknowns > 0:
		out.Status = ReadinessInsufficientEvidence
	case out.Warnings > 0:
		out.Status = ReadinessHighRisk
	default:
		out.Status = ReadinessReady
	}
	return out
}

func combineReadiness(existing, operational Readiness) Readiness {
	if existing.Status == "" || existing.Status == ReadinessReady {
		return operational
	}
	if operational.Status == ReadinessReady {
		return existing
	}
	combined := Readiness{
		Blockers: existing.Blockers + operational.Blockers,
		Warnings: existing.Warnings + operational.Warnings,
		Unknowns: existing.Unknowns + operational.Unknowns,
	}
	if existing.Status == ReadinessBlocked || operational.Status == ReadinessBlocked {
		combined.Status = ReadinessBlocked
		return combined
	}
	if existing.Status == ReadinessInsufficientEvidence || operational.Status == ReadinessInsufficientEvidence {
		combined.Status = ReadinessInsufficientEvidence
		return combined
	}
	combined.Status = ReadinessHighRisk
	return combined
}

func maxCheckStatus(a, b CheckStatus) CheckStatus {
	if checkRank(b) > checkRank(a) {
		return b
	}
	return a
}

func checkRank(status CheckStatus) int {
	switch status {
	case CheckFail:
		return 3
	case CheckUnknown:
		return 2
	case CheckWarning:
		return 1
	default:
		return 0
	}
}

func appendUniqueReason(reasons []ReasonCode, reason ReasonCode) []ReasonCode {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

func hasUnknownCheck(checks []Check) bool {
	for _, check := range checks {
		if check.Status == CheckUnknown {
			return true
		}
	}
	return false
}

func findingCount(fs []findings.Finding, prefix string) int {
	count := 0
	for _, f := range fs {
		if strings.HasPrefix(f.RuleID, prefix) {
			count++
		}
	}
	return count
}

func emptyAsUnknown(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

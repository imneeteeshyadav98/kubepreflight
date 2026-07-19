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

	checks := []Check{
		nodegroupRollbackCheck(assessment, report),
		selfManagedNodeEvidenceCheck(report),
		fargateEvidenceCheck(report),
		managedAddonCheck(report),
		selfManagedAddonCheck(report),
		workloadHealthCheck(report),
		disruptionCheck(report),
		reverseCompatibilityCheck(report),
		coverageCheck(report),
	}
	assessment.Checks = append(assessment.Checks, checks...)
	assessment.Readiness = combineReadiness(assessment.Readiness, readinessFromChecks(checks))
	if hasUnknownCheck(checks) || assessment.Readiness.Status == ReadinessInsufficientEvidence {
		assessment.Evidence.Complete = false
	}
	return assessment
}

func nodegroupRollbackCheck(assessment Assessment, report *findings.Report) Check {
	check := Check{
		ID:     "managed-nodegroups",
		Title:  "Managed node groups are compatible with rollback target",
		Status: CheckPass,
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

func managedAddonCheck(report *findings.Report) Check {
	check := Check{
		ID:     "managed-addons",
		Title:  "EKS managed add-ons are compatible with rollback target",
		Status: CheckPass,
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

func selfManagedAddonCheck(report *findings.Report) Check {
	check := Check{
		ID:     "self-managed-addons",
		Title:  "Self-managed add-on rollback compatibility is verified",
		Status: CheckPass,
	}
	applyFindingSignals(&check, report.Findings, []string{"ADDON-002"}, ReasonSelfManagedAddonCompatibilityUnknown)
	if check.Status == CheckPass {
		check.Evidence = []string{"No self-managed add-on compatibility warnings present"}
	}
	return check
}

func workloadHealthCheck(report *findings.Report) Check {
	check := Check{
		ID:     "workload-health",
		Title:  "Workloads are healthy before rollback",
		Status: CheckPass,
	}
	applyFindingSignals(&check, report.Findings, []string{"WORKLOAD-001", "DRAIN-005"}, ReasonUnhealthyWorkloadsPresent)
	if check.Status == CheckPass {
		check.Evidence = []string{"No unhealthy workload findings present"}
	}
	return check
}

func disruptionCheck(report *findings.Report) Check {
	check := Check{
		ID:     "disruption-readiness",
		Title:  "PDB and drain constraints do not block rollback preparation",
		Status: CheckPass,
	}
	applyFindingSignals(&check, report.Findings, []string{"PDB-", "DRAIN-"}, ReasonPDBDisruptionConstraints)
	if check.Status == CheckPass {
		check.Evidence = []string{"No PDB or drain readiness findings present"}
	}
	return check
}

func reverseCompatibilityCheck(report *findings.Report) Check {
	check := Check{
		ID:     "reverse-compatibility",
		Title:  "API, CRD, and webhook state is compatible with rollback target",
		Status: CheckPass,
	}
	applyFindingSignals(&check, report.Findings, []string{"API-001", "API-002"}, ReasonNewVersionAPIAdoptionRisk)
	applyFindingSignals(&check, report.Findings, []string{"CRD-", "WH-"}, ReasonCRDWebhookControllerRisk)
	if check.Status == CheckPass {
		check.Evidence = []string{"No API, CRD, or webhook rollback compatibility findings present"}
	}
	return check
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

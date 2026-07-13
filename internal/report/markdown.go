package report

import (
	"fmt"
	"io"
	"strings"

	"kubepreflight/internal/findings"
)

// WriteMarkdown renders the same Report data as WriteTerminal — identical
// grouping and Next Actions dedup (view.go) — as plain Markdown suitable
// for pasting into a PR or ticket.
func WriteMarkdown(r *findings.Report, w io.Writer) error {
	var sb strings.Builder

	providerLabel := r.Provider
	if providerLabel == "" {
		providerLabel = "cluster-only"
	}

	fmt.Fprintf(&sb, "# KubePreflight Scan Report\n\n")
	fmt.Fprintf(&sb, "| | |\n|---|---|\n")
	fmt.Fprintf(&sb, "| **Cluster** | %s |\n", orDash(r.ClusterContext))
	fmt.Fprintf(&sb, "| **Target version** | %s |\n", r.TargetVersion)
	fmt.Fprintf(&sb, "| **Provider** | %s |\n", providerLabel)
	if len(r.NamespaceAllowlist) > 0 {
		fmt.Fprintf(&sb, "| **Namespace allowlist** | %s |\n", strings.Join(r.NamespaceAllowlist, ", "))
	}
	fmt.Fprintf(&sb, "| **Scanned at** | %s |\n", r.ScannedAt.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(&sb, "| **Result** | **%s** |\n", r.Result())
	fmt.Fprintf(&sb, "| **Summary** | %d blocker(s), %d warning(s), %d info(s) |\n\n", r.Summary.Blockers, r.Summary.Warnings, r.Summary.Infos)
	if r.CurrentVersion != "" && !r.UpgradeApplicable() {
		fmt.Fprintf(&sb, "> **No version upgrade required:** cluster is already running Kubernetes %s (target: %s). "+
			"Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.\n\n",
			r.CurrentVersion, r.TargetVersion)
	}
	if !r.IsComplete() {
		fmt.Fprintln(&sb, "> **Assessment incomplete:** one or more evidence sources could not be collected; absence of findings is not proof of readiness.")
		fmt.Fprintln(&sb)
		for _, item := range coverageIssueLines(r) {
			fmt.Fprintf(&sb, "- %s\n", item)
		}
		fmt.Fprintln(&sb)
	}
	for _, assumption := range r.Assumptions {
		fmt.Fprintf(&sb, "> **Assumption:** %s\n\n", assumption)
	}
	writeMarkdownUpgradeReadiness(&sb, r.UpgradeReadiness, r.UpgradeApplicable())
	writeMarkdownAPICompatibility(&sb, r.APICompatibility, r.UpgradeApplicable())

	blockers := filterAndSort(r.Findings, findings.SeverityBlocker)
	warnings := filterAndSort(r.Findings, findings.SeverityWarning)

	writeMarkdownSection(&sb, "Blockers", blockers)
	writeMarkdownSection(&sb, "Warnings", warnings)
	writeMarkdownSection(&sb, "Info", filterAndSort(r.Findings, findings.SeverityInfo))

	actionable := make([]findings.Finding, 0, len(blockers)+len(warnings))
	actionable = append(actionable, blockers...)
	actionable = append(actionable, warnings...)
	writeMarkdownNextActions(&sb, buildNextActions(actionable))

	writeMarkdownAppendix(&sb, r.Findings)

	_, err := w.Write([]byte(sb.String()))
	return err
}

func writeMarkdownUpgradeReadiness(sb *strings.Builder, summary *findings.UpgradeReadinessSummary, upgradeApplicable bool) {
	if summary == nil {
		return
	}
	heading, continueLabel, continueValue := "Upgrade Readiness", "Upgrade continue", summary.UpgradeContinue
	if !upgradeApplicable {
		heading, continueLabel, continueValue = "Cluster Health (no version upgrade assessed)", "Remediation needed", !summary.UpgradeContinue
	}
	fmt.Fprintf(sb, "## %s\n\n", heading)
	fmt.Fprintf(sb, "| | |\n|---|---|\n")
	fmt.Fprintf(sb, "| **Verdict** | %s |\n", summary.Verdict)
	fmt.Fprintf(sb, "| **Readiness score** | %d/100 |\n", summary.ReadinessScore)
	fmt.Fprintf(sb, "| **%s** | %s |\n\n", continueLabel, yesNo(continueValue))
	fmt.Fprintf(sb, "| Category | Status | Blockers | Warnings | Rule IDs |\n")
	fmt.Fprintf(sb, "|---|---|---|---|---|\n")
	for _, cat := range summary.Categories {
		fmt.Fprintf(sb, "| %s | %s | %d | %d | %s |\n", cat.Name, cat.Status, cat.BlockerCount, cat.WarningCount, strings.Join(cat.RuleIDs, ", "))
	}
	fmt.Fprintln(sb)
}

func writeMarkdownAPICompatibility(sb *strings.Builder, summary *findings.APICompatibilitySummary, upgradeApplicable bool) {
	if summary == nil {
		return
	}
	continueLabel, continueValue := "Upgrade continue", summary.UpgradeContinue
	if !upgradeApplicable {
		continueLabel, continueValue = "Remediation needed", !summary.UpgradeContinue
	}
	fmt.Fprintf(sb, "## API Compatibility\n\n")
	fmt.Fprintf(sb, "| | |\n|---|---|\n")
	fmt.Fprintf(sb, "| **Status** | %s |\n", summary.Status)
	fmt.Fprintf(sb, "| **%s** | %s |\n", continueLabel, yesNo(continueValue))
	fmt.Fprintf(sb, "| **Score impact** | %d |\n", summary.ScoreImpact)
	fmt.Fprintf(sb, "| **Removed API objects** | %d across %d API %s |\n", summary.RemovedObjects, len(summary.RemovedFamilies), pluralize(len(summary.RemovedFamilies), "family", "families"))
	fmt.Fprintf(sb, "| **Deprecated API objects** | %d across %d API %s |\n", summary.DeprecatedObjects, len(summary.DeprecatedFamilies), pluralize(len(summary.DeprecatedFamilies), "family", "families"))
	fmt.Fprintf(sb, "| **Critical impact** | %s |\n\n", yesNo(summary.CriticalImpact))
	writeMarkdownAPICompatibilityFamilies(sb, "Removed API families", summary.RemovedFamilies)
	writeMarkdownAPICompatibilityFamilies(sb, "Deprecated API families", summary.DeprecatedFamilies)
}

func writeMarkdownAPICompatibilityFamilies(sb *strings.Builder, title string, families []findings.APICompatibilityItem) {
	if len(families) == 0 {
		return
	}
	fmt.Fprintf(sb, "### %s\n\n", title)
	fmt.Fprintf(sb, "| API version | Kind | Objects |\n")
	fmt.Fprintf(sb, "|---|---|---|\n")
	for _, family := range families {
		fmt.Fprintf(sb, "| %s | %s | %d |\n", family.APIVersion, family.Kind, family.Count)
	}
	fmt.Fprintln(sb)
}

func writeMarkdownSection(sb *strings.Builder, title string, fs []findings.Finding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(sb, "## %s (%d)\n\n", title, len(fs))
	for _, f := range fs {
		fmt.Fprintf(sb, "### `%s` `%s` %s\n\n", f.Priority, f.RuleID, f.Message)
		fmt.Fprintf(sb, "Confidence: `%s` · Can upgrade continue: %s\n\n", f.Confidence, yesNo(f.CanUpgradeContinue))
		if f.PriorityReason != "" {
			fmt.Fprintf(sb, "> **Why this matters (%s):** %s\n\n", f.Priority, f.PriorityReason)
		}
		if len(f.Evidence) > 0 {
			fmt.Fprintf(sb, "**Evidence:**\n\n")
			for _, e := range f.Evidence {
				fmt.Fprintf(sb, "- %s\n", e)
			}
			fmt.Fprintln(sb)
		}
		if f.Remediation != "" {
			fmt.Fprintf(sb, "**Remediation:**\n\n```\n%s\n```\n\n", f.Remediation)
		}
	}
}

func writeMarkdownNextActions(sb *strings.Builder, actions []NextAction) {
	if len(actions) == 0 {
		return
	}
	fmt.Fprintf(sb, "## Next Actions (%d)\n\n", len(actions))
	for i, a := range actions {
		fmt.Fprintf(sb, "%d. **[%s/%s] %s** (%s)\n\n", i+1, a.Primary.Priority, a.Severity, a.ResourceLabel, strings.Join(a.RuleIDs, ", "))
		fmt.Fprintf(sb, "   ```\n")
		writeIndentedLines(sb, "   ", a.Primary.Remediation)
		fmt.Fprintf(sb, "   ```\n\n")
		for _, f := range a.Related {
			fmt.Fprintf(sb, "   Also see `%s`: %s\n\n", f.RuleID, firstLine(f.Remediation))
		}
	}
}

func writeMarkdownAppendix(sb *strings.Builder, fs []findings.Finding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(sb, "## Evidence Appendix\n\n")
	fmt.Fprintf(sb, "Every finding's resource identity and fingerprint — cross-reference by fingerprint for waivers/dedup.\n\n")
	fmt.Fprintf(sb, "| Priority | Rule ID | Severity | Confidence | Resource | Fingerprint |\n")
	fmt.Fprintf(sb, "|---|---|---|---|---|---|\n")
	for _, f := range allSorted(fs) {
		fmt.Fprintf(sb, "| %s | %s | %s | %s | %s | `%s` |\n", f.Priority, f.RuleID, f.Severity, f.Confidence, findingResourceLabel(f), f.Fingerprint)
	}
	fmt.Fprintln(sb)
}

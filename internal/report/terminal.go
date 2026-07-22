package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// writeIndentedLines writes text's lines prefixed for indentation, skipping
// the prefix on lines that are already empty — a multi-line Remediation
// (e.g. WH-002's "Step 1 / Step 2" split) uses blank lines as visual
// separators, and prefixing those too leaves indent-only trailing
// whitespace on an otherwise-blank line. Shared by terminal and markdown
// rendering (both package report), not just terminal.go.
func writeIndentedLines(sb *strings.Builder, prefix, text string) {
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			sb.WriteByte('\n')
			continue
		}
		fmt.Fprintf(sb, "%s%s\n", prefix, line)
	}
}

// WriteTerminal renders a human-readable terminal report: a summary header,
// Blockers and Warnings grouped and sorted by rule ID (then resource name)
// for stable, diffable output, and a Next Actions section that
// cross-references findings by resource. This is the layout report.md and
// report.html mirror — same grouping and dedup logic (view.go), different
// formatting only.
func WriteTerminal(r *findings.Report, w io.Writer) error {
	var sb strings.Builder

	providerLabel := r.Provider
	if providerLabel == "" {
		providerLabel = "cluster-only"
	}
	clusterName, _ := clusterDisplayName(r)
	fmt.Fprintf(&sb, "KubePreflight scan — cluster: %s  target: %s  provider: %s  upgrade context: %s\n", orDash(clusterName), r.TargetVersion, providerLabel, orDash(string(r.UpgradeContext)))
	if len(r.NamespaceAllowlist) > 0 {
		fmt.Fprintf(&sb, "Namespace allowlist: %s\n", strings.Join(r.NamespaceAllowlist, ", "))
	}
	fmt.Fprintf(&sb, "Result: %s\n\n", r.Result())
	writeTerminalNoUpgradeNotice(&sb, r)
	writeTerminalCoverage(&sb, r)
	for _, assumption := range r.Assumptions {
		fmt.Fprintf(&sb, "Assumption: %s\n", assumption)
	}
	if len(r.Assumptions) > 0 {
		fmt.Fprintln(&sb)
	}
	writeTerminalUpgradeReadiness(&sb, r.UpgradeReadiness, r.UpgradeApplicable())
	writeTerminalAPICompatibility(&sb, r.APICompatibility, r.UpgradeApplicable())

	findingIndex := newReportFindingIndex(r.Findings)
	blockers := findingIndex.severity(findings.SeverityBlocker)
	warnings := findingIndex.severity(findings.SeverityWarning)

	writeTerminalSection(&sb, "Blockers", blockers)
	writeTerminalSection(&sb, "Warnings", warnings)
	writeTerminalSection(&sb, "Info", findingIndex.severity(findings.SeverityInfo))

	writeTerminalNextActions(&sb, buildNextActionsFromMetas(findingIndex.actionableMetas()), r.UpgradeApplicable())

	fmt.Fprintf(&sb, "Summary: %d blocker(s), %d warning(s), %d operator decision(s), %d info(s)\n", r.Summary.Blockers, r.Summary.Warnings, r.Summary.OperatorDecisions, r.Summary.Infos)

	_, err := w.Write([]byte(sb.String()))
	return err
}

func writeTerminalCoverage(sb *strings.Builder, r *findings.Report) {
	if r.IsComplete() {
		return
	}
	if r.Summary.Blockers > 0 {
		fmt.Fprintf(sb, "ASSESSMENT INCOMPLETE — %d blocker(s) observed with available evidence; absence of findings elsewhere is not proof of readiness.\n", r.Summary.Blockers)
	} else {
		fmt.Fprintln(sb, "ASSESSMENT INCOMPLETE — absence of findings is not proof of readiness.")
	}
	for _, item := range coverageIssueLines(r) {
		fmt.Fprintf(sb, "  - %s\n", item)
	}
	fmt.Fprintln(sb)
}

// writeTerminalNoUpgradeNotice prints a notice when CurrentVersion and
// TargetVersion are known and resolve to the same release: there's no
// version transition being assessed, so the "Upgrade Readiness" framing
// below would otherwise misleadingly imply an upgrade decision is being
// made. Silent whenever UpgradeApplicable() is true, including when
// CurrentVersion is unknown -- see its doc comment for why "don't know" and
// "definitely different" both fall back to today's plain framing.
func writeTerminalNoUpgradeNotice(sb *strings.Builder, r *findings.Report) {
	if r.CurrentVersion == "" || r.UpgradeApplicable() {
		return
	}
	fmt.Fprintf(sb, "NO VERSION UPGRADE REQUIRED — cluster is already running Kubernetes %s (target: %s). "+
		"Upgrade-transition checks were skipped; current-state and manifest-safety findings below were still fully evaluated.\n\n",
		r.CurrentVersion, r.TargetVersion)
}

func writeTerminalUpgradeReadiness(sb *strings.Builder, summary *findings.UpgradeReadinessSummary, upgradeApplicable bool) {
	if summary == nil {
		return
	}
	heading, continueLabel, continueValue := "Upgrade Readiness", "Upgrade Continue", summary.UpgradeContinue
	if !upgradeApplicable {
		// Same "is this healthy enough to act on" fact, reframed: with no
		// version transition happening, "Upgrade Continue" doesn't apply,
		// but whether remediation is needed before the *next* real upgrade
		// still does -- UpgradeContinue==false means categories failed,
		// i.e. remediation is needed.
		heading, continueLabel, continueValue = "Cluster Health (no version upgrade assessed)", "Remediation Needed", !summary.UpgradeContinue
	}
	fmt.Fprintf(sb, "%s: %s — Score: %d/100 — %s: %s\n", heading, summary.Verdict, summary.ReadinessScore, continueLabel, yesNo(continueValue))
	for _, cat := range summary.Categories {
		fmt.Fprintf(sb, "  %s: %s (%d blocker(s), %d warning(s))\n", cat.Name, cat.Status, cat.BlockerCount, cat.WarningCount)
	}
	fmt.Fprintln(sb)
}

func writeTerminalAPICompatibility(sb *strings.Builder, summary *findings.APICompatibilitySummary, upgradeApplicable bool) {
	if summary == nil {
		return
	}
	continueLabel, continueValue := "Upgrade Continue", summary.UpgradeContinue
	if !upgradeApplicable {
		continueLabel, continueValue = "Remediation Needed", !summary.UpgradeContinue
	}
	fmt.Fprintf(sb, "API Compatibility: %s — %s: %s — Score Impact: %d\n", summary.Status, continueLabel, yesNo(continueValue), summary.ScoreImpact)
	fmt.Fprintf(sb, "  Removed API objects: %d across %d API %s\n", summary.RemovedObjects, len(summary.RemovedFamilies), pluralize(len(summary.RemovedFamilies), "family", "families"))
	fmt.Fprintf(sb, "  Deprecated API objects: %d across %d API %s\n", summary.DeprecatedObjects, len(summary.DeprecatedFamilies), pluralize(len(summary.DeprecatedFamilies), "family", "families"))
	fmt.Fprintf(sb, "  Critical impact: %s\n\n", yesNo(summary.CriticalImpact))
}

// WriteCompactSummary renders the short form of the terminal report used
// when the local report server is active: report.html and the Console
// already show every finding's evidence and remediation, so printing the
// full per-finding detail to stdout too is redundant noise on top of the
// server URLs. Deliberately excludes Blockers/Warnings/Next Actions detail
// — see internal/cli/scan.go's --terminal-output flag for how callers pick
// this over WriteTerminal.
func WriteCompactSummary(r *findings.Report, w io.Writer) error {
	var sb strings.Builder

	providerLabel := r.Provider
	if providerLabel == "" {
		providerLabel = "cluster-only"
	}
	clusterName, _ := clusterDisplayName(r)
	fmt.Fprintf(&sb, "Scan complete — cluster: %s  target: %s  provider: %s  upgrade context: %s\n", orDash(clusterName), r.TargetVersion, providerLabel, orDash(string(r.UpgradeContext)))
	fmt.Fprintf(&sb, "Result: %s\n", r.Result())
	fmt.Fprintf(&sb, "Blockers: %d  Warnings: %d  Operator decisions: %d  Info: %d\n", r.Summary.Blockers, r.Summary.Warnings, r.Summary.OperatorDecisions, r.Summary.Infos)

	_, err := w.Write([]byte(sb.String()))
	return err
}

func writeTerminalSection(sb *strings.Builder, title string, fs []findings.Finding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(sb, "%s (%d)\n", title, len(fs))
	for _, f := range fs {
		fmt.Fprintf(sb, "  [%s/%s] %s\n", f.Priority, f.RuleID, f.Message)
		if f.PriorityReason != "" {
			continueLine := "can continue upgrade planning"
			if !f.CanUpgradeContinue {
				continueLine = "do not attempt other remediation until this is fixed"
			}
			fmt.Fprintf(sb, "    Priority %s (%s): %s\n", f.Priority, continueLine, f.PriorityReason)
		}
		fmt.Fprintf(sb, "    Upgrade gate: %s", f.EffectiveUpgradeGate())
		if len(f.ImpactScopes) > 0 {
			fmt.Fprintf(sb, " · Impact scope: %s", impactScopesLabel(f.ImpactScopes))
		}
		fmt.Fprintln(sb)
		if len(f.Evidence) > 0 {
			fmt.Fprintf(sb, "    Evidence:\n")
			for _, e := range f.Evidence {
				fmt.Fprintf(sb, "      - %s\n", e)
			}
		}
		if f.Remediation != "" {
			fmt.Fprintf(sb, "    Remediation:\n")
			writeIndentedLines(sb, "      ", f.Remediation)
		}
		fmt.Fprintln(sb)
	}
}

func writeTerminalNextActions(sb *strings.Builder, actions []NextAction, upgradeApplicable bool) {
	if len(actions) == 0 {
		return
	}
	heading := "Next Actions"
	if !upgradeApplicable {
		heading = "Recommended Maintenance"
	}
	fmt.Fprintf(sb, "%s (%d)\n", heading, len(actions))
	for i, a := range actions {
		fmt.Fprintf(sb, "  %d. [%s/%s] %s (%s)\n", i+1, a.Primary.Priority, a.Severity, a.ResourceLabel, strings.Join(a.RuleIDs, ", "))
		writeIndentedLines(sb, "     ", a.Primary.Remediation)
		for _, f := range a.Related {
			fmt.Fprintf(sb, "     Also see %s: %s\n", f.RuleID, firstLine(f.Remediation))
		}
		fmt.Fprintln(sb)
	}
}

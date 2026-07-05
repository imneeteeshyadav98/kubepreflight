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

func writeMarkdownSection(sb *strings.Builder, title string, fs []findings.Finding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(sb, "## %s (%d)\n\n", title, len(fs))
	for _, f := range fs {
		fmt.Fprintf(sb, "### `%s` %s\n\n", f.RuleID, f.Message)
		fmt.Fprintf(sb, "Confidence: `%s`\n\n", f.Confidence)
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
		fmt.Fprintf(sb, "%d. **[%s] %s** (%s)\n\n", i+1, a.Severity, a.ResourceLabel, strings.Join(a.RuleIDs, ", "))
		fmt.Fprintf(sb, "   ```\n")
		for _, line := range strings.Split(a.Primary.Remediation, "\n") {
			fmt.Fprintf(sb, "   %s\n", line)
		}
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
	fmt.Fprintf(sb, "| Rule ID | Severity | Confidence | Resource | Fingerprint |\n")
	fmt.Fprintf(sb, "|---|---|---|---|---|\n")
	for _, f := range allSorted(fs) {
		fmt.Fprintf(sb, "| %s | %s | %s | %s | `%s` |\n", f.RuleID, f.Severity, f.Confidence, findingResourceLabel(f), f.Fingerprint)
	}
	fmt.Fprintln(sb)
}

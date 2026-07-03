package report

import (
	"fmt"
	"io"
	"strings"

	"kubepreflight/internal/findings"
)

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
	fmt.Fprintf(&sb, "KubePreflight scan — cluster: %s  target: %s  provider: %s\n", orDash(r.ClusterContext), r.TargetVersion, providerLabel)
	if len(r.NamespaceAllowlist) > 0 {
		fmt.Fprintf(&sb, "Namespace allowlist: %s\n", strings.Join(r.NamespaceAllowlist, ", "))
	}
	fmt.Fprintf(&sb, "Result: %s\n\n", r.Result())
	for _, assumption := range r.Assumptions {
		fmt.Fprintf(&sb, "Assumption: %s\n", assumption)
	}
	if len(r.Assumptions) > 0 {
		fmt.Fprintln(&sb)
	}

	blockers := filterAndSort(r.Findings, findings.SeverityBlocker)
	warnings := filterAndSort(r.Findings, findings.SeverityWarning)

	writeTerminalSection(&sb, "Blockers", blockers)
	writeTerminalSection(&sb, "Warnings", warnings)

	actionable := make([]findings.Finding, 0, len(blockers)+len(warnings))
	actionable = append(actionable, blockers...)
	actionable = append(actionable, warnings...)
	writeTerminalNextActions(&sb, buildNextActions(actionable))

	fmt.Fprintf(&sb, "Summary: %d blocker(s), %d warning(s), %d info(s)\n", r.Summary.Blockers, r.Summary.Warnings, r.Summary.Infos)

	_, err := w.Write([]byte(sb.String()))
	return err
}

func writeTerminalSection(sb *strings.Builder, title string, fs []findings.Finding) {
	if len(fs) == 0 {
		return
	}
	fmt.Fprintf(sb, "%s (%d)\n", title, len(fs))
	for _, f := range fs {
		fmt.Fprintf(sb, "  [%s] %s\n", f.RuleID, f.Message)
		if len(f.Evidence) > 0 {
			fmt.Fprintf(sb, "    Evidence:\n")
			for _, e := range f.Evidence {
				fmt.Fprintf(sb, "      - %s\n", e)
			}
		}
		if f.Remediation != "" {
			fmt.Fprintf(sb, "    Remediation:\n")
			for _, line := range strings.Split(f.Remediation, "\n") {
				fmt.Fprintf(sb, "      %s\n", line)
			}
		}
		fmt.Fprintln(sb)
	}
}

func writeTerminalNextActions(sb *strings.Builder, actions []NextAction) {
	if len(actions) == 0 {
		return
	}
	fmt.Fprintf(sb, "Next Actions (%d)\n", len(actions))
	for i, a := range actions {
		fmt.Fprintf(sb, "  %d. [%s] %s (%s)\n", i+1, a.Severity, a.ResourceLabel, strings.Join(a.RuleIDs, ", "))
		for _, line := range strings.Split(a.Primary.Remediation, "\n") {
			fmt.Fprintf(sb, "     %s\n", line)
		}
		for _, f := range a.Related {
			fmt.Fprintf(sb, "     Also see %s: %s\n", f.RuleID, firstLine(f.Remediation))
		}
		fmt.Fprintln(sb)
	}
}

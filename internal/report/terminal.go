package report

import (
	"fmt"
	"io"
	"strings"

	"kubepreflight/internal/findings"
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
	fmt.Fprintf(&sb, "KubePreflight scan — cluster: %s  target: %s  provider: %s\n", orDash(r.ClusterContext), r.TargetVersion, providerLabel)
	if len(r.NamespaceAllowlist) > 0 {
		fmt.Fprintf(&sb, "Namespace allowlist: %s\n", strings.Join(r.NamespaceAllowlist, ", "))
	}
	fmt.Fprintf(&sb, "Result: %s\n\n", r.Result())
	writeTerminalCoverage(&sb, r)
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
	writeTerminalSection(&sb, "Info", filterAndSort(r.Findings, findings.SeverityInfo))

	actionable := make([]findings.Finding, 0, len(blockers)+len(warnings))
	actionable = append(actionable, blockers...)
	actionable = append(actionable, warnings...)
	writeTerminalNextActions(&sb, buildNextActions(actionable))

	fmt.Fprintf(&sb, "Summary: %d blocker(s), %d warning(s), %d info(s)\n", r.Summary.Blockers, r.Summary.Warnings, r.Summary.Infos)

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
	fmt.Fprintf(&sb, "Scan complete — cluster: %s  target: %s  provider: %s\n", orDash(r.ClusterContext), r.TargetVersion, providerLabel)
	fmt.Fprintf(&sb, "Result: %s\n", r.Result())
	fmt.Fprintf(&sb, "Blockers: %d  Warnings: %d  Info: %d\n", r.Summary.Blockers, r.Summary.Warnings, r.Summary.Infos)

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

func writeTerminalNextActions(sb *strings.Builder, actions []NextAction) {
	if len(actions) == 0 {
		return
	}
	fmt.Fprintf(sb, "Next Actions (%d)\n", len(actions))
	for i, a := range actions {
		fmt.Fprintf(sb, "  %d. [%s/%s] %s (%s)\n", i+1, a.Primary.Priority, a.Severity, a.ResourceLabel, strings.Join(a.RuleIDs, ", "))
		writeIndentedLines(sb, "     ", a.Primary.Remediation)
		for _, f := range a.Related {
			fmt.Fprintf(sb, "     Also see %s: %s\n", f.RuleID, firstLine(f.Remediation))
		}
		fmt.Fprintln(sb)
	}
}

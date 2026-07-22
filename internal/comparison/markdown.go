package comparison

import (
	"fmt"
	"io"
	"strings"
)

// WriteMarkdown renders a Comparison as a change-ticket-pasteable summary,
// following the same shape internal/report/action_plan.go's
// WriteActionPlanMarkdown already established for kubepreflight's other
// standalone Markdown documents.
func WriteMarkdown(c *Comparison, w io.Writer) error {
	if c == nil {
		return fmt.Errorf("write comparison markdown: comparison is nil")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Upgrade Readiness Comparison\n\n")
	fmt.Fprintf(&sb, "- Schema version: `%s`\n", c.SchemaVersion)
	for _, warning := range c.Warnings {
		fmt.Fprintf(&sb, "- **Warning:** %s\n", warning)
	}
	fmt.Fprintln(&sb)

	s := c.Summary
	verdictArrow := s.BaselineVerdict
	if s.VerdictChanged {
		verdictArrow = fmt.Sprintf("%s → %s", s.BaselineVerdict, s.CurrentVerdict)
	}
	fmt.Fprintf(&sb, "| | |\n|---|---|\n")
	fmt.Fprintf(&sb, "| **Verdict** | %s |\n", verdictArrow)
	fmt.Fprintf(&sb, "| **Upgrade context** | %s → %s |\n", s.BaselineUpgradeContext, s.CurrentUpgradeContext)
	fmt.Fprintf(&sb, "| **Readiness score** | %d → %d (%s) |\n", s.BaselineReadinessScore, s.CurrentReadinessScore, signedDelta(s.ReadinessScoreDelta))
	fmt.Fprintf(&sb, "| **New** | %d (%d blocker(s)) |\n", s.New, s.NewBlockers)
	fmt.Fprintf(&sb, "| **Resolved** | %d (%d blocker(s)) |\n", s.Resolved, s.ResolvedBlockers)
	fmt.Fprintf(&sb, "| **Changed** | %d |\n", s.Changed)
	fmt.Fprintf(&sb, "| **Unchanged** | %d |\n\n", s.Unchanged)

	writeEntrySection(&sb, "New findings", c.New)
	writeChangedSection(&sb, c.Changed)
	writeEntrySection(&sb, "Resolved findings", c.Resolved)
	writeEntrySection(&sb, "Unchanged findings", c.Unchanged)

	_, err := io.WriteString(w, sb.String())
	return err
}

func signedDelta(delta int) string {
	if delta > 0 {
		return fmt.Sprintf("+%d", delta)
	}
	return fmt.Sprintf("%d", delta)
}

func writeEntrySection(sb *strings.Builder, title string, entries []Entry) {
	fmt.Fprintf(sb, "## %s (%d)\n\n", title, len(entries))
	if len(entries) == 0 {
		fmt.Fprintf(sb, "None.\n\n")
		return
	}
	fmt.Fprintf(sb, "| Priority | Severity | Rule | Resource | Message |\n|---|---|---|---|---|\n")
	for _, e := range entries {
		ns, name := firstResourceIdentity(e.Resources)
		resource := name
		if ns != "" {
			resource = ns + "/" + name
		}
		fmt.Fprintf(sb, "| %s | %s | `%s` | %s | %s |\n", e.Priority, e.Severity, e.RuleID, resource, e.Message)
	}
	fmt.Fprintln(sb)
}

func writeChangedSection(sb *strings.Builder, changed []Changed) {
	fmt.Fprintf(sb, "## Changed findings (%d)\n\n", len(changed))
	if len(changed) == 0 {
		fmt.Fprintf(sb, "None.\n\n")
		return
	}
	for _, c := range changed {
		ns, name := firstResourceIdentity(c.Resources)
		resource := name
		if ns != "" {
			resource = ns + "/" + name
		}
		fmt.Fprintf(sb, "- **`%s`** %s\n", c.RuleID, resource)
		for _, field := range []string{"severity", "priority", "confidence", "canUpgradeContinue", "affectedScope", "ruleId", "resource"} {
			if fc, ok := c.Changes[field]; ok {
				fmt.Fprintf(sb, "  - %s: `%s` → `%s`\n", field, fc.Before, fc.After)
			}
		}
	}
	fmt.Fprintln(sb)
}

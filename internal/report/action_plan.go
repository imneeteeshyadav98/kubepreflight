package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/plan"
)

func WriteActionPlanJSON(actionPlan *plan.UpgradeActionPlan, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(actionPlan)
}

func WriteActionPlanMarkdown(actionPlan *plan.UpgradeActionPlan, w io.Writer) error {
	if actionPlan == nil {
		return fmt.Errorf("write action plan markdown: action plan is nil")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Upgrade Execution Plan\n\n")
	fmt.Fprintf(&sb, "- Schema version: `%s`\n", actionPlan.SchemaVersion)
	fmt.Fprintf(&sb, "- Verdict: `%s`\n", actionPlan.Verdict)
	fmt.Fprintf(&sb, "- Generated at: `%s`\n\n", actionPlan.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))

	fmt.Fprintf(&sb, "## Change Ticket Checklist\n\n")
	for _, phase := range actionPlan.Phases {
		fmt.Fprintf(&sb, "### %s\n\n", phase.Title)
		if phase.Description != "" {
			fmt.Fprintf(&sb, "%s\n\n", phase.Description)
		}
		if phase.Gate != "" {
			fmt.Fprintf(&sb, "**Gate:** %s\n\n", phase.Gate)
		}
		if len(phase.Actions) == 0 {
			fmt.Fprintf(&sb, "- [x] No required actions detected for this phase.\n\n")
			continue
		}
		for _, action := range phase.Actions {
			required := "optional"
			if action.Required {
				required = "required"
			}
			fmt.Fprintf(&sb, "- [ ] **%s** (`%s`, %s)\n", action.Title, action.Status, required)
			if action.Reason != "" {
				fmt.Fprintf(&sb, "  - Why: %s\n", action.Reason)
			}
			if len(action.SourceRuleIDs) > 0 {
				fmt.Fprintf(&sb, "  - Source rules: `%s`\n", strings.Join(action.SourceRuleIDs, "`, `"))
			}
			for _, criterion := range action.SuccessCriteria {
				fmt.Fprintf(&sb, "  - Success: %s\n", criterion)
			}
			for _, command := range action.Commands {
				fmt.Fprintf(&sb, "  - Command: `%s`\n", command)
			}
		}
		fmt.Fprintln(&sb)
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

package report

import (
	"fmt"
	"io"

	"kubepreflight/internal/plan"
)

// htmlPlanOverview is the Upgrade Path section's data — nil on every
// scan-produced htmlViewData (see html.go), populated only by
// WritePlanHTML.
type htmlPlanOverview struct {
	FromVersion, ToVersion                    string
	VerdictLabel, VerdictClass, VerdictReason string
	Hops                                      []htmlPlanHop
}

// htmlPlanHop is one row in the Upgrade Path list. Result/ResultClass are
// only meaningful for the EXACT hop (hop 1) — predicted hops show
// Blockers/Warnings (if any findings were honestly re-evaluated) and a
// Rescan-required badge with the carry-forward reasons instead.
type htmlPlanHop struct {
	From, To                 string
	StatusLabel, StatusClass string
	Result, ResultClass      string
	Blockers, Warnings       int
	CarryForward             []string
	RescanRequired           bool
}

// WritePlanHTML renders a plan.PlanReport as the same command-center HTML
// page WriteHTML produces for a single scan (identical Summary/Blockers/
// Warnings/Next Actions/Evidence from hop 1's report — see
// buildHTMLViewData), with an added Upgrade Path section: the readiness
// verdict plus one row per hop with Current-live/Projected/Rescan-required
// badges.
func WritePlanHTML(p *plan.PlanReport, w io.Writer) error {
	if len(p.Hops) == 0 || p.Hops[0].Report == nil {
		return fmt.Errorf("write plan HTML: plan has no hop 1 report")
	}

	data := buildHTMLViewData(p.Hops[0].Report)
	data.Plan = buildPlanOverview(p)
	return htmlTmpl.Execute(w, data)
}

func buildPlanOverview(p *plan.PlanReport) *htmlPlanOverview {
	label, reason := p.Verdict()
	overview := &htmlPlanOverview{
		FromVersion:   p.FromVersion,
		ToVersion:     p.ToVersion,
		VerdictLabel:  label,
		VerdictClass:  verdictClass(label),
		VerdictReason: reason,
		Hops:          make([]htmlPlanHop, 0, len(p.Hops)),
	}

	for _, hr := range p.Hops {
		hop := htmlPlanHop{From: hr.Hop.From, To: hr.Hop.To}
		switch hr.Status {
		case plan.HopStatusExact:
			hop.StatusLabel, hop.StatusClass = "Current live", "current-live"
			if hr.Report != nil {
				hop.Result = hr.Report.Result()
				hop.ResultClass = resultClass(hr.Report.Result())
			}
		case plan.HopStatusPredicted:
			hop.StatusLabel, hop.StatusClass = "Projected", "projected"
			if hr.Report != nil {
				hop.Blockers = hr.Report.Summary.Blockers
				hop.Warnings = hr.Report.Summary.Warnings
			}
		}
		for _, note := range hr.CarryForward {
			hop.CarryForward = append(hop.CarryForward, fmt.Sprintf("%s: %s", note.RuleID, note.Reason))
		}
		hop.RescanRequired = len(hop.CarryForward) > 0
		overview.Hops = append(overview.Hops, hop)
	}

	return overview
}

// verdictClass maps a Verdict() label to the same blocked/warn/clean CSS
// classes resultClass already uses, so the verdict banner shares the
// report's existing color language instead of inventing a new one.
func verdictClass(label string) string {
	switch label {
	case "NOT READY FOR UPGRADE":
		return "blocked"
	case "CONDITIONALLY READY":
		return "warn"
	default:
		return "clean"
	}
}

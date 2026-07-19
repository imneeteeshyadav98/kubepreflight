// plan_terminal.go is kept separate from terminal.go so existing
// scan-only code doesn't gain a new dependency on internal/plan — only
// this file (and plan_json.go) import it.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/imneeteeshyadav98/kubepreflight/internal/plan"
)

// WritePlanCompactSummary renders the short form of a `plan` run's terminal
// output: cluster/current/target/provider, the immediate next hop's real
// result, and the full hop path with each hop's status. Deliberately
// excludes per-finding Evidence/Remediation detail — report.html/the
// Console already show that, same rationale as WriteCompactSummary.
func WritePlanCompactSummary(p *plan.PlanReport, w io.Writer) error {
	var sb strings.Builder

	providerLabel := p.Provider
	if providerLabel == "" {
		providerLabel = "cluster-only"
	}
	fmt.Fprintf(&sb, "Plan complete — cluster: %s  current: %s  target: %s  provider: %s\n",
		orDash(p.ClusterContext), p.FromVersion, p.ToVersion, providerLabel)

	if len(p.Hops) > 0 && p.Hops[0].Report != nil {
		hop1 := p.Hops[0]
		fmt.Fprintf(&sb, "Next hop (%s -> %s): %s — Result: %s (%d blocker(s), %d warning(s), %d info(s))\n",
			hop1.Hop.From, hop1.Hop.To, hop1.Status, hop1.Report.Result(),
			hop1.Report.Summary.Blockers, hop1.Report.Summary.Warnings, hop1.Report.Summary.Infos)
	}

	fmt.Fprintln(&sb, "Path:")
	for _, hr := range p.Hops {
		fmt.Fprintf(&sb, "  %d. %s -> %s  %-11s %s\n", hr.Hop.Index, hr.Hop.From, hr.Hop.To, hr.Status, planHopDetail(hr))
	}

	_, err := w.Write([]byte(sb.String()))
	return err
}

// planHopDetail renders the short trailing description for one hop's path
// line: the real Result() for the exact hop, or a predicted-findings/
// carry-forward-risk count for future hops.
func planHopDetail(hr plan.HopReport) string {
	switch hr.Status {
	case plan.HopStatusExact:
		if hr.Report == nil {
			return ""
		}
		return hr.Report.Result()
	case plan.HopStatusPredicted:
		var parts []string
		if hr.Report != nil && len(hr.Report.Findings) > 0 {
			parts = append(parts, fmt.Sprintf("%d blocker(s), %d warning(s) predicted", hr.Report.Summary.Blockers, hr.Report.Summary.Warnings))
		}
		if len(hr.CarryForward) > 0 {
			parts = append(parts, fmt.Sprintf("%d check(s) require a rescan", len(hr.CarryForward)))
		}
		if len(parts) == 0 {
			return "no predicted risks"
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

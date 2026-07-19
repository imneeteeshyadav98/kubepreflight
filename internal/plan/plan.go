package plan

import (
	"fmt"
	"time"

	"github.com/imneeteeshyadav98/kubepreflight/internal/findings"
)

// HopStatus classifies how a HopReport's content was produced.
type HopStatus string

const (
	// HopStatusExact marks the immediate next hop: a real scan, exactly
	// like `scan --target-version <hop.To>` would produce.
	HopStatusExact HopStatus = "EXACT"
	// HopStatusPredicted marks a future hop: only rule categories that can
	// be honestly re-evaluated (see classify.go) are included as findings;
	// everything else is a CarryForwardNote, never a fabricated finding.
	HopStatusPredicted HopStatus = "PREDICTED"
)

// CarryForwardNote documents one rule category that is NOT projected
// forward for a given hop, and why — surfaced instead of a fabricated
// finding so a future hop never overstates certainty about live-cluster
// state that will likely have changed by the time that hop is reached.
type CarryForwardNote struct {
	RuleID             string `json:"ruleId"`
	Reason             string `json:"reason"`
	RecommendedCommand string `json:"recommendedCommand"`
}

// HopReport is one hop in the plan.
//   - Status == HopStatusExact (hop 1 only): Report is a full, real
//     *findings.Report built the same way `scan` builds one.
//   - Status == HopStatusPredicted (hop 2+): Report holds only the
//     findings from rule categories honestly re-evaluated for this hop;
//     CarryForward lists every other rule category with a rescan note
//     instead of a projected finding.
type HopReport struct {
	Hop          Hop                `json:"hop"`
	Status       HopStatus          `json:"status"`
	Report       *findings.Report   `json:"report,omitempty"`
	CarryForward []CarryForwardNote `json:"carryForward,omitempty"`
}

// PlanReport is the top-level upgrade-plan.json document.
type PlanReport struct {
	SchemaVersion     string             `json:"schemaVersion"`
	ClusterContext    string             `json:"clusterContext,omitempty"`
	Provider          string             `json:"provider,omitempty"`
	FromVersion       string             `json:"fromVersion"`
	FromVersionSource string             `json:"fromVersionSource"`
	ToVersion         string             `json:"toVersion"`
	GeneratedAt       time.Time          `json:"generatedAt"`
	Hops              []HopReport        `json:"hops"`
	ActionPlan        *UpgradeActionPlan `json:"actionPlan,omitempty"`
}

// BuildPlan assembles a PlanReport from a pre-computed hop-1 findings.Report
// (the real scan, built the same way scan.go builds one) plus a
// caller-supplied per-hop assessment function for hops 2..N. It performs no
// I/O itself — internal/cli owns collecting evidence and calling AWS/k8s;
// this mirrors how rules.Registry.RunAll takes a pre-built ScanContext
// rather than collecting its own evidence.
func BuildPlan(
	clusterContext, provider, fromVersion, fromVersionSource, toVersion string,
	hops []Hop,
	hop1Report *findings.Report,
	assessFutureHop func(hop Hop) (HopReport, error),
	now time.Time,
) (*PlanReport, error) {
	if len(hops) == 0 {
		return nil, fmt.Errorf("BuildPlan: no hops given")
	}
	if hop1Report == nil {
		return nil, fmt.Errorf("BuildPlan: hop1Report is required")
	}

	hopReports := make([]HopReport, 0, len(hops))
	hopReports = append(hopReports, HopReport{
		Hop:    hops[0],
		Status: HopStatusExact,
		Report: hop1Report,
	})

	for _, hop := range hops[1:] {
		hr, err := assessFutureHop(hop)
		if err != nil {
			return nil, fmt.Errorf("assessing hop %s -> %s: %w", hop.From, hop.To, err)
		}
		hopReports = append(hopReports, hr)
	}

	return &PlanReport{
		SchemaVersion:     findings.SchemaVersion,
		ClusterContext:    clusterContext,
		Provider:          provider,
		FromVersion:       fromVersion,
		FromVersionSource: fromVersionSource,
		ToVersion:         toVersion,
		GeneratedAt:       now,
		Hops:              hopReports,
		ActionPlan:        BuildActionPlan(hop1Report, now),
	}, nil
}

// OverallExitCode mirrors the immediate next hop's real scan result only —
// future hops are advisory/predicted and never affect the process
// exit-code contract (0 clean, 1 warnings only, 2 blockers present).
func (p *PlanReport) OverallExitCode() int {
	if len(p.Hops) == 0 || p.Hops[0].Report == nil {
		return 0
	}
	return p.Hops[0].Report.ExitCode()
}

// Verdict returns a one-line upgrade-readiness decision and its reason,
// derived only from hop 1 (the real, current-live scan) — matching
// OverallExitCode's rule that predicted hops never drive the headline
// decision. Priority order matches Report.Result()/ExitCode() exactly, so
// the plan verdict, the CLI exit code, and the scan-level result can never
// disagree: incomplete coverage outranks even a global blocker finding —
// an assessment built on partial evidence isn't a fully-trusted "not
// ready" verdict either, though any blockers found with the evidence that
// WAS collected are still named in the reason, never hidden.
func (p *PlanReport) Verdict() (label, reason string) {
	if len(p.Hops) == 0 || p.Hops[0].Report == nil {
		return "READY", "No known upgrade blockers detected"
	}
	r := p.Hops[0].Report

	if !r.IsComplete() {
		if r.Summary.Blockers > 0 {
			return "ASSESSMENT INCOMPLETE", fmt.Sprintf(
				"Assessment incomplete; %d blocker(s) observed with available evidence. One or more evidence sources could not be collected — resolve coverage errors and rerun.",
				r.Summary.Blockers)
		}
		return "ASSESSMENT INCOMPLETE", "One or more evidence sources could not be collected; resolve coverage errors and rerun"
	}

	for _, f := range r.Findings {
		if f.GlobalBlocker {
			return "NOT READY FOR UPGRADE", "Global API write blocker detected"
		}
	}
	switch r.Result() {
	case "BLOCKED":
		return "NOT READY FOR UPGRADE", fmt.Sprintf("%d blocker(s) found", r.Summary.Blockers)
	case "PASSED_WITH_WARNINGS":
		return "CONDITIONALLY READY", fmt.Sprintf("No hard blockers, but %d warning(s) should be reviewed", r.Summary.Warnings)
	default:
		return "READY", "No known upgrade blockers detected"
	}
}

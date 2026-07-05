package report

import (
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"

	"kubepreflight/internal/findings"
)

// html/template is deliberate, not text/template: remediation text
// contains raw shell placeholder syntax like `--cluster-name <cluster>`
// and `kubectl convert -f <file> --output-version <group>/<version>`.
// Rendered through text/template or naive string concatenation, browsers
// would interpret <cluster> and <file> as unknown HTML tags and silently
// drop them from the page. html/template's contextual auto-escaping is
// what keeps the rendered report byte-faithful to the actual finding data.
var htmlTmpl = template.Must(template.New("report").Parse(htmlTemplateSource))

type htmlFinding struct {
	findings.Finding
	ResourceLabel string
	PlaneLabel    string
	// ElementID is a per-rendered-instance-unique DOM id base (e.g.
	// "blocker-3"), used to build unique ids for this finding's copy-target
	// <pre> elements. Empty where no remediation panel is rendered (top
	// risks, evidence appendix).
	ElementID string
	// DependencyWarning is true when this finding's own remediation
	// commands may fail until a separate global blocker elsewhere in the
	// report is fixed first — never true for the global blocker's own
	// finding, and never true when the report has no global blocker at all.
	DependencyWarning bool
}

// SeverityClass renders the finding's severity as the lowercase CSS class
// (blocker/warning/info) used throughout the template's severity styling.
func (f htmlFinding) SeverityClass() string { return strings.ToLower(string(f.Severity)) }

type htmlRelatedNote struct {
	RuleID string
	Note   string
}

type htmlNextAction struct {
	ResourceLabel string
	RuleIDsJoined string
	Severity      findings.Severity
	Remediation   string
	Related       []htmlRelatedNote
	// ElementID is a per-rendered-instance-unique DOM id base, mirroring
	// htmlFinding.ElementID, for this action's own copy-target <pre>.
	ElementID string
	// GroupedPlan is a synthesized, numbered remediation plan for a merged
	// group (2+ findings sharing a resource) — nil for a single-finding
	// group, where the template falls back to the plain Remediation <pre>.
	GroupedPlan []string
}

// SeverityClass renders the action's severity as the lowercase CSS class
// (blocker/warning/info) used throughout the template's severity styling.
func (a htmlNextAction) SeverityClass() string { return strings.ToLower(string(a.Severity)) }

type htmlConfidenceStat struct {
	Tier  findings.ConfidenceTier
	Count int
}

type htmlTopRisk struct {
	htmlFinding
	Rank int
}

type htmlViewData struct {
	Cluster             string
	Target              string
	Provider            string
	AWSEnrichment       bool
	NamespaceAllowlist  string
	ScannedAt           string
	Result              string
	ResultClass         string
	Decision            string
	WhyLine             string
	Blockers            int
	Warnings            int
	Infos               int
	TotalFindings       int
	GlobalBlockerCount  int
	Assumptions         []string
	ConfidenceMix       []htmlConfidenceStat
	TopRisks            []htmlTopRisk
	BlockerFindings     []htmlFinding
	WarningFindings     []htmlFinding
	NextActions         []htmlNextAction
	NextActionsPreview  []htmlNextAction
	NextActionsOverflow int
	AllFindings         []htmlFinding
	// Plan is nil for every scan-produced report (WriteHTML never sets
	// it) — the template's {{if .Plan}} Upgrade Path section stays
	// hidden. Only WritePlanHTML (plan_html.go) populates it.
	Plan *htmlPlanOverview
}

// WriteHTML renders the same Report data as WriteTerminal — identical
// grouping and Next Actions dedup (view.go) — as a standalone HTML file:
// inline CSS and a small vanilla-JS filter/search + tab-switching pass, no
// external assets, no build step, no CDN dependency. Still a single
// self-contained file: works as a CAB-ticket attachment or an offline
// double-click open with no internet access needed to view or interact
// with it. Screen view is a compact single-page command center — Summary/
// Findings/Next actions/Evidence behind tabs, only one visible at a time —
// while printing expands every section (see the beforeprint handler)
// since a physical CAB packet has no tabs to click. The visual language
// (navy banner, eyebrow labels, metric cards, severity/confidence pills,
// GO/REVIEW/NO-GO decision framing) intentionally mirrors the local
// Console (web/) so the CAB-style static report and the interactive
// viewer read as the same product.
func WriteHTML(r *findings.Report, w io.Writer) error {
	data := buildHTMLViewData(r)
	return htmlTmpl.Execute(w, data)
}

// buildHTMLViewData builds the Summary/Blockers/Warnings/Next Actions/
// Evidence view data shared by WriteHTML (scan) and WritePlanHTML (plan)
// — both render this identically from a single findings.Report (hop 1's
// report, for plan); WritePlanHTML additionally sets the returned
// htmlViewData's Plan field afterward, which WriteHTML never does.
func buildHTMLViewData(r *findings.Report) htmlViewData {
	providerLabel := r.Provider
	if providerLabel == "" {
		providerLabel = "cluster-only"
	}

	blockers := filterAndSort(r.Findings, findings.SeverityBlocker)
	warnings := filterAndSort(r.Findings, findings.SeverityWarning)

	actionable := make([]findings.Finding, 0, len(blockers)+len(warnings))
	actionable = append(actionable, blockers...)
	actionable = append(actionable, warnings...)

	nextActions := toHTMLNextActions(buildNextActions(actionable))
	preview := nextActions
	overflow := 0
	if len(nextActions) > 3 {
		preview = nextActions[:3]
		overflow = len(nextActions) - 3
	}

	globalBlockerCount := 0
	for _, f := range r.Findings {
		if f.GlobalBlocker {
			globalBlockerCount++
		}
	}
	hasGlobalBlocker := globalBlockerCount > 0

	return htmlViewData{
		Cluster:             orDash(r.ClusterContext),
		Target:              r.TargetVersion,
		Provider:            providerLabel,
		AWSEnrichment:       awsEnrichment(r),
		NamespaceAllowlist:  strings.Join(r.NamespaceAllowlist, ", "),
		ScannedAt:           r.ScannedAt.Format("2006-01-02 15:04:05 MST"),
		Result:              r.Result(),
		ResultClass:         resultClass(r.Result()),
		Decision:            decisionLabel(r.Result()),
		WhyLine:             decisionWhyLine(r.Summary.Blockers, r.Summary.Warnings),
		Blockers:            r.Summary.Blockers,
		Warnings:            r.Summary.Warnings,
		Infos:               r.Summary.Infos,
		TotalFindings:       len(r.Findings),
		GlobalBlockerCount:  globalBlockerCount,
		Assumptions:         r.Assumptions,
		ConfidenceMix:       confidenceMix(r.Findings),
		TopRisks:            toHTMLTopRisks(topRisks(r.Findings, 3)),
		BlockerFindings:     toHTMLFindings(blockers, "blocker", hasGlobalBlocker),
		WarningFindings:     toHTMLFindings(warnings, "warning", hasGlobalBlocker),
		NextActions:         nextActions,
		NextActionsPreview:  preview,
		NextActionsOverflow: overflow,
		AllFindings:         toHTMLFindings(allSorted(r.Findings), "all", hasGlobalBlocker),
	}
}

func resultClass(result string) string {
	switch result {
	case "BLOCKED":
		return "blocked"
	case "PASSED_WITH_WARNINGS":
		return "warn"
	default:
		return "clean"
	}
}

// decisionLabel/decisionWhyLine are display-only derivations layered on top
// of Result/Summary — GO/REVIEW/NO-GO is a presentation label for report
// readers (mirrors web/src/lib/findings-schema.ts's decisionFromResult on
// the Console side), not a new machine-readable field. The authoritative
// value stays Result (CLEAN/PASSED_WITH_WARNINGS/BLOCKED) and the CLI exit
// code, both unchanged.
func decisionLabel(result string) string {
	switch result {
	case "BLOCKED":
		return "NO-GO"
	case "PASSED_WITH_WARNINGS":
		return "REVIEW"
	default:
		return "GO"
	}
}

func decisionWhyLine(blockers, warnings int) string {
	switch {
	case blockers > 0:
		plural := "s"
		if blockers == 1 {
			plural = ""
		}
		return fmt.Sprintf("%d blocker%s found — fix required before the change window.", blockers, plural)
	case warnings > 0:
		plural := "s"
		if warnings == 1 {
			plural = ""
		}
		return fmt.Sprintf("%d warning%s found — review before the change window.", warnings, plural)
	default:
		return "No blockers or warnings — safe to proceed."
	}
}

// topRisks: highest-severity findings first (ties broken by rule ID, then
// resource), truncated to limit — the same "worst findings first" order as
// every other renderer, just capped for an above-the-fold executive
// summary. Not a scoring model.
func topRisks(fs []findings.Finding, limit int) []findings.Finding {
	sorted := make([]findings.Finding, len(fs))
	copy(sorted, fs)
	sort.Slice(sorted, func(i, j int) bool {
		bi, bj := sorted[i].GlobalBlocker, sorted[j].GlobalBlocker
		if bi != bj {
			return bi // global blockers always sort first, ahead of severity
		}
		ri, rj := severityRank(sorted[i].Severity), severityRank(sorted[j].Severity)
		if ri != rj {
			return ri < rj
		}
		if sorted[i].RuleID != sorted[j].RuleID {
			return sorted[i].RuleID < sorted[j].RuleID
		}
		return findingResourceLabel(sorted[i]) < findingResourceLabel(sorted[j])
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

// awsEnrichment mirrors the Console's own rule (web/app.mjs): true when the
// scan explicitly used the eks provider, or any finding carries evidence
// collected from the AWS plane — so a cluster-only scan that happens to hit
// an AWS-tagged finding (shouldn't happen, but would be surprising if
// silently labeled "false") is still reported honestly.
func awsEnrichment(r *findings.Report) bool {
	if r.Provider == "eks" {
		return true
	}
	for _, f := range r.Findings {
		for _, ref := range f.Resources {
			if ref.Plane == findings.PlaneAWS {
				return true
			}
		}
	}
	return false
}

func confidenceMix(fs []findings.Finding) []htmlConfidenceStat {
	counts := map[findings.ConfidenceTier]int{}
	for _, f := range fs {
		counts[f.Confidence]++
	}
	order := []findings.ConfidenceTier{findings.TierStaticCertain, findings.TierProviderReported}
	seen := map[findings.ConfidenceTier]bool{}
	var out []htmlConfidenceStat
	for _, tier := range order {
		if counts[tier] > 0 {
			out = append(out, htmlConfidenceStat{Tier: tier, Count: counts[tier]})
		}
		seen[tier] = true
	}
	var rest []findings.ConfidenceTier
	for tier := range counts {
		if !seen[tier] {
			rest = append(rest, tier)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i] < rest[j] })
	for _, tier := range rest {
		out = append(out, htmlConfidenceStat{Tier: tier, Count: counts[tier]})
	}
	return out
}

func toHTMLFindings(fs []findings.Finding, elementIDPrefix string, hasGlobalBlocker bool) []htmlFinding {
	out := make([]htmlFinding, len(fs))
	for i, f := range fs {
		out[i] = htmlFinding{
			Finding:           f,
			ResourceLabel:     findingResourceLabel(f),
			PlaneLabel:        planeLabel(f),
			ElementID:         fmt.Sprintf("%s-%d", elementIDPrefix, i),
			DependencyWarning: hasGlobalBlocker && !f.GlobalBlocker && hasLiveResource(f) && hasRemediationCommand(f),
		}
	}
	return out
}

// hasLiveResource reports whether the finding references at least one
// live-cluster resource — a manifest-only fix (editing a local YAML file)
// isn't blocked by a cluster-side admission webhook, so it never gets the
// "this command may fail" dependency warning.
func hasLiveResource(f findings.Finding) bool {
	for _, ref := range f.Resources {
		if ref.Plane == findings.PlaneLive {
			return true
		}
	}
	return false
}

// hasRemediationCommand reports whether the finding has a real
// copy-pastable command a dependency warning would even apply to.
func hasRemediationCommand(f findings.Finding) bool {
	if f.RemediationDetail == nil {
		return false
	}
	if f.RemediationDetail.SafeFix != nil && f.RemediationDetail.SafeFix.Command != "" {
		return true
	}
	if f.RemediationDetail.Emergency != nil && f.RemediationDetail.Emergency.Command != "" {
		return true
	}
	return false
}

func toHTMLTopRisks(fs []findings.Finding) []htmlTopRisk {
	out := make([]htmlTopRisk, len(fs))
	for i, f := range fs {
		out[i] = htmlTopRisk{
			htmlFinding: htmlFinding{Finding: f, ResourceLabel: findingResourceLabel(f), PlaneLabel: planeLabel(f)},
			Rank:        i + 1,
		}
	}
	return out
}

func planeLabel(f findings.Finding) string {
	seen := map[findings.Plane]bool{}
	var planes []string
	for _, ref := range f.Resources {
		if !seen[ref.Plane] {
			seen[ref.Plane] = true
			planes = append(planes, string(ref.Plane))
		}
	}
	return strings.Join(planes, " + ")
}

func toHTMLNextActions(actions []NextAction) []htmlNextAction {
	out := make([]htmlNextAction, len(actions))
	for i, a := range actions {
		var related []htmlRelatedNote
		for _, f := range a.Related {
			related = append(related, htmlRelatedNote{RuleID: f.RuleID, Note: firstLine(f.Remediation)})
		}

		var groupedPlan []string
		if len(a.Related) > 0 {
			groupedPlan = append(groupedPlan, groupedPlanStep(a.Primary))
			for _, f := range a.Related {
				groupedPlan = append(groupedPlan, groupedPlanStep(f))
			}
			groupedPlan = append(groupedPlan, "Verify the fix and rerun `kubepreflight scan` to confirm the blocker clears.")
		}

		out[i] = htmlNextAction{
			ResourceLabel: a.ResourceLabel,
			RuleIDsJoined: strings.Join(a.RuleIDs, ", "),
			Severity:      a.Severity,
			Remediation:   a.Primary.Remediation,
			Related:       related,
			ElementID:     fmt.Sprintf("action-%d", i),
			GroupedPlan:   groupedPlan,
		}
	}
	return out
}

// groupedPlanStep renders one finding as a single actionable step for a
// merged Next Action's grouped plan: the structured safe-fix command when
// available, falling back to the plain remediation text's first line for
// findings without a RemediationDetail.
func groupedPlanStep(f findings.Finding) string {
	if f.RemediationDetail != nil && f.RemediationDetail.SafeFix != nil {
		if f.RemediationDetail.SafeFix.Command != "" {
			return fmt.Sprintf("[%s] %s", f.RuleID, f.RemediationDetail.SafeFix.Command)
		}
		if len(f.RemediationDetail.SafeFix.Steps) > 0 {
			return fmt.Sprintf("[%s] %s", f.RuleID, f.RemediationDetail.SafeFix.Steps[0])
		}
	}
	return fmt.Sprintf("[%s] %s", f.RuleID, firstLine(f.Remediation))
}

const htmlTemplateSource = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>KubePreflight Scan Report — {{.Cluster}}</title>
<style>
  :root {
    --ink: #17221f;
    --muted: #66706c;
    --paper: #f3f1ea;
    --surface: #fffdf8;
    --line: #d8d8cf;
    --navy: #102c30;
    --navy-soft: #1a3d40;
    --mint: #b8dfcf;
    --red: #c5483d;
    --red-soft: #f6ded9;
    --amber: #a96f13;
    --amber-soft: #f7e8c8;
    --blue: #235b70;
    --blue-soft: #dcebf0;
    --shadow: 0 16px 50px rgba(16, 44, 48, .1);
    --shadow-card: 0 1px 2px rgba(16, 44, 48, .05), 0 6px 16px rgba(16, 44, 48, .06);
    --radius: 10px;
    --radius-sm: 6px;
  }
  * { box-sizing: border-box; }
  body {
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    color: var(--ink);
    background: var(--paper);
    width: min(100% - 48px, 1600px);
    margin: 0 auto;
    padding: 0 0 60px;
    line-height: 1.5;
    font-size: 16px;
  }
  code, pre, .eyebrow, .badge, .severity-pill, .confidence-pill, .rule-id, .decision-label { font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace; }
  .eyebrow { margin: 0; color: var(--blue); font-size: 10px; font-weight: 700; letter-spacing: .14em; text-transform: uppercase; }
  h1 { margin: 6px 0 0; font: 600 clamp(22px, 3.6vw, 30px)/1.15 Inter, ui-sans-serif, system-ui, sans-serif; letter-spacing: -.02em; }
  h2.section-title { margin: 0 0 12px; font: 700 20px/1.3 Inter, ui-sans-serif, system-ui, sans-serif; border-bottom: 1px solid var(--line); padding-bottom: 6px; }
  h3 { font-size: 15px; margin: 0; }
  h4 { margin: 0 0 6px; font-size: 10.5px; text-transform: uppercase; letter-spacing: .08em; color: var(--muted); }

  .banner { margin-top: 20px; padding: 20px 24px; background: var(--navy); color: white; border-radius: var(--radius); box-shadow: var(--shadow); }
  .banner .eyebrow { color: var(--mint); }
  .banner-top-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
  .console-link { display: inline-flex; align-items: center; padding: 8px 14px; border: 1px solid rgba(255,255,255,.35); border-radius: var(--radius-sm); color: white; font-size: 12px; font-weight: 700; text-decoration: none; white-space: nowrap; }
  .console-link:hover { background: rgba(255,255,255,.12); }
  .decision-row { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; margin-top: 8px; }
  .decision-mark { display: grid; place-items: center; min-width: 100px; height: 56px; padding: 0 14px; border: 2px solid currentColor; border-radius: var(--radius-sm); flex-shrink: 0; }
  .decision-mark.blocked { color: #ffaaa1; } .decision-mark.warn { color: #ffd28c; } .decision-mark.clean { color: var(--mint); }
  .decision-label { font: 700 18px/1 monospace; letter-spacing: .03em; }
  .decision-copy { flex: 1 1 280px; min-width: 220px; }
  .decision-copy h1 { color: white; font-size: clamp(18px, 3vw, 24px); }
  .why-line { margin: 6px 0 0; color: #dfeae6; font-size: 14px; }
  .banner-meta { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; margin: 16px 0 0; padding-top: 14px; border-top: 1px solid rgba(255,255,255,.14); }
  .meta-chip { padding: 9px 12px; border: 1px solid rgba(255,255,255,.14); border-radius: var(--radius-sm); background: rgba(255,255,255,.04); }
  .banner-meta dt { color: #8ca49e; font-size: 10px; text-transform: uppercase; letter-spacing: .1em; }
  .banner-meta dd { margin: 4px 0 0; font: 13px monospace; }

  .badge { display: inline-block; padding: 6px 9px; border: 1px solid currentColor; border-radius: var(--radius-sm); font-size: 10.5px; font-weight: 700; letter-spacing: .08em; }
  .badge.blocked { color: #ffaaa1; } .badge.warn { color: #ffd28c; } .badge.clean { color: var(--mint); }

  .summary-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 12px; }
  .metric { padding: 14px 16px; border: 1px solid var(--line); border-top: 3px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); }
  .metric span { display: block; color: var(--muted); font-size: 10.5px; text-transform: uppercase; letter-spacing: .06em; }
  .metric strong { display: block; margin: 6px 0 0; font-size: 26px; }
  .metric small { display: block; margin-top: 4px; color: var(--muted); font-size: 11.5px; }
  .metric-blocker { border-top-color: var(--red); } .metric-blocker strong { color: var(--red); }
  .metric-warning { border-top-color: var(--amber); } .metric-warning strong { color: var(--amber); }
  .metric-info { border-top-color: var(--blue); } .metric-info strong { color: var(--blue); }

  /* Tabs: the compact single-page layout. Only one .tab-panel is visible
     at a time on screen (toggled by the inline script below); printing
     forces every panel open (see the beforeprint handler) since a
     physical CAB packet has no tabs to click. */
  .tab-nav { display: flex; gap: 4px; margin-top: 16px; padding: 4px; background: #ece9df; border-radius: var(--radius); }
  .tab-button { padding: 8px 16px; border: 0; border-radius: var(--radius-sm); background: none; color: var(--muted); font-size: 13.5px; font-weight: 700; cursor: pointer; transition: background-color .1s, color .1s; }
  .tab-button:hover { color: var(--ink); background: rgba(255,255,255,.6); }
  .tab-button:focus-visible { outline: 2px solid var(--navy); outline-offset: 2px; }
  .tab-button.tab-active { color: var(--ink); background: var(--surface); box-shadow: var(--shadow-card); }
  .tab-count { padding: 1px 6px; border-radius: 8px; background: #eceae0; font-size: 10px; font-weight: 700; margin-left: 4px; }
  .tab-button.tab-active .tab-count { background: var(--navy); color: white; }
  .tab-panel { padding-top: 14px; }
  .tab-panel.hidden { display: none; }
  .tab-panel > section + section, .tab-panel > .assumptions { margin-top: 14px; }

  .plan-verdict-banner { border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); padding: 14px 16px; }
  .plan-verdict-banner h2 { margin: 0; font: 700 16px/1.3 Inter, ui-sans-serif, system-ui, sans-serif; }
  .plan-verdict-banner p { margin: 6px 0 0; font-size: 13.5px; color: var(--muted); }
  .plan-verdict-banner.blocked { border-color: var(--red); background: var(--red-soft); } .plan-verdict-banner.blocked h2 { color: #8e2d25; }
  .plan-verdict-banner.warn { border-color: var(--amber); background: var(--amber-soft); } .plan-verdict-banner.warn h2 { color: #754706; }
  .plan-verdict-banner.clean { border-color: var(--mint); background: #e3f5ee; } .plan-verdict-banner.clean h2 { color: #146c50; }
  .upgrade-path-list { list-style: none; margin: 10px 0 0; padding: 0; display: grid; gap: 8px; }
  .hop-row { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 10px 14px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); font-size: 13.5px; }
  .hop-versions { font-weight: 700; font-family: monospace; }
  .hop-counts { color: var(--muted); }
  .badge-current-live, .badge-projected, .badge-rescan-required { display: inline-flex; align-items: center; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; }
  .badge-current-live { background: var(--mint); color: #0c3d2c; }
  .badge-projected { background: var(--blue-soft); color: var(--blue); }
  .badge-rescan-required { background: var(--amber-soft); color: #754706; }
  .badge-blocked { background: var(--red-soft); color: #8e2d25; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .badge-warn { background: var(--amber-soft); color: #754706; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .badge-clean { background: #e3f5ee; color: #146c50; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .carry-forward-list { flex: 1 1 100%; margin: 4px 0 0; padding-left: 18px; font-size: 12.5px; color: var(--muted); }
  .upgrade-path-caption { margin: 10px 0 0; font-size: 12.5px; color: var(--muted); }

  .global-blocker-banner { border: 1px solid var(--red); border-left: 4px solid var(--red); border-radius: var(--radius); background: var(--red-soft); padding: 14px 16px; }
  .global-blocker-banner h2 { margin: 0; font: 700 15px/1.3 Inter, ui-sans-serif, system-ui, sans-serif; color: #8e2d25; }
  .global-blocker-banner p { margin: 6px 0 0; font-size: 13.5px; color: #6b241d; }
  .global-blocker-count { font-weight: 700; }
  .global-blocker-badge { display: inline-flex; align-items: center; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; background: var(--red); color: white; }
  .dependency-warning { margin: 6px 0 0; padding: 6px 10px; font-size: 12.5px; color: #6b241d; background: var(--red-soft); border-left: 3px solid var(--red); }

  .top-risks-list { list-style: none; margin: 10px 0 0; padding: 0; display: grid; gap: 8px; }
  .top-risks-list li { display: flex; align-items: baseline; flex-wrap: wrap; gap: 4px 10px; padding: 10px 14px; border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); font-size: 14px; }
  .top-risks-list li.blocker { border-left-color: var(--red); }
  .top-risks-list li.warning { border-left-color: var(--amber); }
  .top-risks-list li.info { border-left-color: var(--blue); }
  .top-risks-list .rank { flex-shrink: 0; display: inline-grid; place-items: center; width: 18px; height: 18px; border-radius: 50%; background: var(--navy); color: white; font: 700 10px monospace; }
  .top-risks-list .rule-id { font-size: 11px; padding: 5px 9px; }
  .top-risks-list .risk-resource { font-weight: 700; min-width: 0; overflow-wrap: anywhere; }
  .top-risks-list .risk-reason { color: var(--muted); min-width: 0; overflow-wrap: anywhere; }

  .preview-actions-list { list-style: none; margin: 10px 0 0; padding: 0; display: grid; gap: 8px; }
  .preview-actions-list li { display: flex; align-items: flex-start; flex-wrap: wrap; gap: 4px 10px; padding: 10px 14px; border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); font-size: 14px; }
  .preview-actions-list li.blocker { border-left-color: var(--red); }
  .preview-actions-list li.warning { border-left-color: var(--amber); }
  .preview-actions-list li.info { border-left-color: var(--blue); }
  .preview-actions-list .risk-resource { font-weight: 700; min-width: 0; overflow-wrap: anywhere; }
  .preview-actions-list .risk-reason { color: var(--muted); flex: 1 1 260px; min-width: 0; overflow: hidden; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
  .view-all-link { display: inline-block; margin-top: 8px; font-size: 13px; font-weight: 700; color: var(--blue); }

  .confidence-panel { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px 24px; padding: 12px 16px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); }
  .confidence-panel .eyebrow { margin-bottom: 4px; }
  .confidence-group + .confidence-group { padding-left: 24px; border-left: 1px solid var(--line); }
  .confidence-list { display: flex; flex-wrap: wrap; gap: 8px; }
  .confidence-stat { display: flex; align-items: center; gap: 8px; padding: 6px 9px; border: 1px solid var(--line); border-radius: var(--radius-sm); font-size: 12.5px; }
  .confidence-stat b { font: 700 13px monospace; }

  .assumptions { padding: 12px 16px; border-left: 3px solid var(--blue); background: var(--blue-soft); font-size: 13.5px; }
  .assumptions p { margin: 4px 0; }

  .toolbar { border: 1px solid var(--line); padding: 10px 14px; margin-bottom: 10px; background: var(--surface); }
  .toolbar-row { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; margin-bottom: 6px; }
  .toolbar-row:last-of-type { margin-bottom: 0; }
  .toolbar-label { font-weight: 600; font-size: 13px; color: var(--muted); }
  .toolbar label { font-size: 13px; display: inline-flex; align-items: center; gap: 4px; cursor: pointer; }
  .toolbar input[type="text"] { padding: 6px 10px; border: 1px solid var(--line); font-size: 13.5px; flex: 1; min-width: 160px; background: white; }
  .toolbar-count { font-size: 12.5px; color: var(--muted); margin-top: 4px; }
  .hidden { display: none !important; }

  .finding-row { border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); margin-bottom: 8px; overflow: hidden; }
  .finding-row.blocker { border-left-color: var(--red); }
  .finding-row.warning { border-left-color: var(--amber); }
  .finding-row summary { display: flex; align-items: flex-start; gap: 10px; flex-wrap: wrap; padding: 12px 16px; cursor: pointer; list-style: none; }
  .finding-row summary::-webkit-details-marker { display: none; }
  .finding-row summary::before { content: "▸"; color: var(--muted); font-size: 10px; flex-shrink: 0; margin-top: 3px; transition: transform .1s; }
  .finding-row[open] summary::before { transform: rotate(90deg); }
  .finding-row summary:hover { background: #f7f6f0; }
  .finding-resource { font-size: 14px; }
  .finding-message { color: var(--muted); font-size: 13.5px; flex: 1 1 260px; min-width: 0; overflow: hidden; overflow-wrap: anywhere; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; }
  .finding-row[open] .finding-message { -webkit-line-clamp: unset; display: block; }
  .finding-body { padding: 4px 16px 16px 32px; }
  .finding-body h4 { margin-top: 10px; }
  .finding-body h4:first-child { margin-top: 0; }
  .finding-body ul { margin: 0; padding-left: 18px; }
  .severity-pill, .confidence-pill, .plane-pill, .rule-id { display: inline-flex; align-items: center; white-space: nowrap; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; }
  .severity-pill.blocker { background: var(--red-soft); color: #8e2d25; }
  .severity-pill.warning { background: var(--amber-soft); color: #754706; }
  .severity-pill.info { background: var(--blue-soft); color: var(--blue); }
  .confidence-pill { border: 1px solid var(--line); color: var(--blue); background: white; }
  .plane-pill { gap: 5px; color: var(--muted); background: #f0efe8; }
  .rule-id { background: #eceae0; color: var(--ink); }
  .finding-row.blocker .rule-id { background: var(--red-soft); color: #8e2d25; }
  .finding-row.warning .rule-id { background: var(--amber-soft); color: #754706; }
  pre { background: #f5f4ee; border: 1px solid var(--line); padding: 10px 12px; overflow-x: auto; font-size: 13.5px; white-space: pre-wrap; word-break: break-word; }
  /* .remediation-panel uses the CSS order property to show a header row —
     label left, button top-right — with the panel's <pre> full-width below.
     Copy buttons target their <pre> via data-copy-target/id (not DOM
     position), so a finding can have several independent panels (diff,
     safe fix, emergency, verify) without ambiguity. */
  .remediation-panel { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; margin-top: 10px; }
  .remediation-panel h4 { order: 1; margin: 0; flex: 1 1 auto; }
  .remediation-panel pre { order: 3; flex: 1 1 100%; margin: 0; }
  .remediation-panel.emergency-panel { border-left: 3px solid var(--amber); background: var(--amber-soft); padding: 10px 12px; }
  .remediation-panel.emergency-panel h4 { color: #754706; }
  .remediation-panel.breakglass-panel { border-left: 3px solid var(--red); background: var(--red-soft); padding: 10px 12px; }
  .remediation-panel.breakglass-panel h4 { color: #8e2d25; }
  .copy-btn { order: 2; margin-top: 0; padding: 6px 12px; border: 1px solid var(--line); background: white; color: var(--blue); font-size: 12px; font-weight: 700; cursor: pointer; }
  .copy-btn:hover { background: var(--blue-soft); }
  .change-required { border-left: 3px solid var(--blue); background: var(--blue-soft); padding: 8px 12px; margin-top: 10px; border-radius: var(--radius); }
  .change-required h4 { margin: 0 0 6px; color: var(--blue); font-size: 12px; text-transform: uppercase; letter-spacing: .04em; }
  .change-row { display: flex; gap: 8px; flex-wrap: wrap; align-items: baseline; font-size: 13.5px; }
  .change-row + .change-row { margin-top: 4px; }
  .change-field { font-weight: 700; min-width: 150px; }
  .change-arrow { color: var(--muted); }
  .expected-result { margin: 6px 0 0; font-size: 13px; color: var(--muted); }
  ol.grouped-plan { margin: 8px 0 0; padding-left: 18px; font-size: 13.5px; }
  ol.grouped-plan li { margin: 2px 0; }

  ol.next-actions { list-style: none; margin: 0; padding: 0; display: grid; gap: 10px; }
  ol.next-actions > li { border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); padding: 14px 16px; overflow-wrap: anywhere; }
  ol.next-actions > li.blocker { border-left-color: var(--red); }
  ol.next-actions > li.warning { border-left-color: var(--amber); }
  ol.next-actions > li.info { border-left-color: var(--blue); }
  .action-head { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin-bottom: 4px; }
  .next-action-heading { overflow-wrap: anywhere; font-size: 14px; }
  .also-see { color: var(--muted); font-size: 13px; margin-top: 8px; }

  .table-wrap { overflow-x: auto; contain: inline-size; border: 1px solid var(--line); border-radius: var(--radius); box-shadow: var(--shadow-card); }
  table.appendix { border-collapse: collapse; width: 100%; font-size: 13.5px; background: var(--surface); }
  table.appendix th, table.appendix td { border: 1px solid var(--line); padding: 9px 12px; text-align: left; }
  table.appendix th { background: #f0efe8; font-size: 10px; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  table.appendix td.fingerprint { font-family: monospace; font-size: 11.5px; word-break: break-all; }

  footer { margin-top: 40px; color: var(--muted); font-size: 13px; border-top: 1px solid var(--line); padding-top: 12px; }

  /* Compact on screen, complete on paper: printing shows every tab panel
     and expands every collapsed finding row (via the beforeprint handler
     below) — the interactive chrome that only makes sense on screen
     (tab nav, filter toolbar) is hidden instead. */
  @media print {
    .screen-only { display: none !important; }
    body { width: auto; max-width: none; }
  }

  @media (max-width: 720px) {
    html { overflow-x: hidden; }
    .tab-nav { overflow-x: auto; flex-wrap: nowrap; }
    .tab-button { flex-shrink: 0; }
    .confidence-group + .confidence-group { padding-left: 0; border-left: none; padding-top: 10px; border-top: 1px solid var(--line); }
  }
</style>
</head>
<body>
  <header class="banner" id="summary">
    <div class="banner-top-row">
      <p class="eyebrow">Upgrade readiness report</p>
      <a href="/console/?findings=/findings.json#summary" class="console-link screen-only">Open Interactive Console</a>
    </div>
    <div class="decision-row">
      <div class="decision-mark {{.ResultClass}}"><span class="decision-label">{{.Decision}}</span></div>
      <div class="decision-copy">
        <h1>KubePreflight Scan Report</h1>
        <span class="badge {{.ResultClass}}">{{.Result}}</span>
        <p class="why-line">{{.WhyLine}}</p>
      </div>
    </div>
    <dl class="banner-meta">
      <div class="meta-chip"><dt>Cluster</dt><dd>{{.Cluster}}</dd></div>
      <div class="meta-chip"><dt>Target version</dt><dd>{{.Target}}</dd></div>
      <div class="meta-chip"><dt>Provider</dt><dd>{{.Provider}}</dd></div>
      <div class="meta-chip"><dt>AWS enrichment</dt><dd>{{.AWSEnrichment}}</dd></div>
      <div class="meta-chip"><dt>Scanned at</dt><dd>{{.ScannedAt}}</dd></div>
      {{if .NamespaceAllowlist}}<div class="meta-chip"><dt>Namespace allowlist</dt><dd>{{.NamespaceAllowlist}}</dd></div>{{end}}
    </dl>
  </header>

  <section class="summary-grid" aria-label="Scan summary">
    <article class="metric metric-blocker"><span>Blockers</span><strong>{{.Blockers}}</strong><small>Must fix before the change window</small></article>
    <article class="metric metric-warning"><span>Warnings</span><strong>{{.Warnings}}</strong><small>Review before proceeding</small></article>
    <article class="metric metric-info"><span>Info</span><strong>{{.Infos}}</strong><small>No action required</small></article>
  </section>

  <nav class="tab-nav screen-only" role="tablist" aria-label="Report sections">
    <button type="button" class="tab-button tab-active" data-tab="summary">Summary</button>
    <button type="button" class="tab-button" data-tab="findings">Findings<span class="tab-count">{{.TotalFindings}}</span></button>
    <button type="button" class="tab-button" data-tab="actions">Next actions<span class="tab-count">{{len .NextActions}}</span></button>
    <button type="button" class="tab-button" data-tab="evidence">Evidence</button>
  </nav>

  <div class="tab-panel" data-panel="summary" id="top-risks">
    {{if .Plan}}
    <section class="plan-verdict-banner {{.Plan.VerdictClass}}">
      <h2>{{.Plan.VerdictLabel}}</h2>
      <p>{{.Plan.VerdictReason}}</p>
    </section>
    <section class="upgrade-path">
      <h2 class="section-title">Upgrade Path ({{.Plan.FromVersion}} &rarr; {{.Plan.ToVersion}})</h2>
      <ol class="upgrade-path-list">
        {{range .Plan.Hops}}
        <li class="hop-row">
          <span class="hop-versions">{{.From}} &rarr; {{.To}}</span>
          <span class="badge-{{.StatusClass}}">{{.StatusLabel}}</span>
          {{if .Result}}<span class="badge-{{.ResultClass}}">{{.Result}}</span>{{end}}
          {{if or .Blockers .Warnings}}<span class="hop-counts">{{.Blockers}} blocker(s), {{.Warnings}} warning(s)</span>{{end}}
          {{if .RescanRequired}}
          <span class="badge-rescan-required">Rescan required</span>
          <ul class="carry-forward-list">{{range .CarryForward}}<li>{{.}}</li>{{end}}</ul>
          {{end}}
        </li>
        {{end}}
      </ol>
      <p class="upgrade-path-caption">Future-hop findings are projections. Re-run <code>kubepreflight scan</code> after each completed upgrade step.</p>
    </section>
    {{end}}

    {{if .GlobalBlockerCount}}
    <section class="global-blocker-banner">
      <h2>Global API write blocker detected</h2>
      <p>This can block kubectl apply, kubectl patch, kubectl scale, Helm upgrades, and other remediation commands. Fix this before attempting other remediation.</p>
      <p class="global-blocker-count">{{.GlobalBlockerCount}} global blocker{{if ne .GlobalBlockerCount 1}}s{{end}} may prevent remediation commands from running.</p>
    </section>
    {{end}}

    {{if .TopRisks}}
    <section>
      <h2 class="section-title">Top risks</h2>
      <ol class="top-risks-list">
        {{range .TopRisks}}
        <li class="{{.SeverityClass}}">
          <span class="rank">{{.Rank}}</span>
          <span class="severity-pill {{.SeverityClass}}">{{.Severity}}</span>
          <span class="rule-id">{{.RuleID}}</span>
          <span class="risk-resource">{{.ResourceLabel}}</span>
          <span class="risk-reason">{{.Message}}</span>
        </li>
        {{end}}
      </ol>
    </section>
    {{end}}

    {{if .NextActionsPreview}}
    <section>
      <h2 class="section-title">Top next actions</h2>
      <ul class="preview-actions-list">
        {{range .NextActionsPreview}}
        <li class="{{.SeverityClass}}">
          <span class="severity-pill {{.SeverityClass}}">{{.Severity}}</span>
          <span class="risk-resource">{{.ResourceLabel}}</span>
          <span class="risk-reason">{{.Remediation}}</span>
        </li>
        {{end}}
      </ul>
      {{if .NextActionsOverflow}}<a class="view-all-link screen-only" data-goto-tab="actions" href="#next-actions">View all {{len .NextActions}} next actions ({{.NextActionsOverflow}} more) →</a>{{end}}
    </section>
    {{end}}

    {{if .ConfidenceMix}}
    <section class="confidence-panel">
      <div class="confidence-group">
        <p class="eyebrow">Confidence mix</p>
        <div class="confidence-list">
          {{range .ConfidenceMix}}<div class="confidence-stat"><b>{{.Count}}</b><span>{{.Tier}}</span></div>{{end}}
        </div>
      </div>
      <div class="confidence-group">
        <p class="eyebrow">Scan source</p>
        <div class="confidence-list">
          <div class="confidence-stat"><span>Provider: {{.Provider}}</span></div>
          <div class="confidence-stat"><span>AWS enrichment: {{if .AWSEnrichment}}on{{else}}off{{end}}</span></div>
        </div>
      </div>
      <div class="confidence-group">
        <p class="eyebrow">Generated</p>
        <div class="confidence-list">
          <div class="confidence-stat"><span>{{.ScannedAt}}</span></div>
        </div>
      </div>
    </section>
    {{end}}

    {{if .Assumptions}}
    <section class="assumptions">
      {{range .Assumptions}}<p><strong>Assumption:</strong> {{.}}</p>{{end}}
    </section>
    {{end}}
  </div>

  <div class="tab-panel hidden" data-panel="findings" id="findings">
    <div class="toolbar screen-only">
      <div class="toolbar-row">
        <span class="toolbar-label">Severity:</span>
        <label><input type="checkbox" class="sev-filter" value="Blocker" checked> Blocker</label>
        <label><input type="checkbox" class="sev-filter" value="Warning" checked> Warning</label>
        <label><input type="checkbox" class="sev-filter" value="Info" checked> Info</label>
      </div>
      <div class="toolbar-row">
        <input type="text" id="rule-filter" placeholder="Filter by rule ID…">
        <input type="text" id="resource-filter" placeholder="Search by resource name…">
      </div>
      <div class="toolbar-count" id="filter-count"></div>
    </div>

    {{define "remediationDetail"}}
    {{with .RemediationDetail}}
    {{if .Changes}}
    <div class="change-required">
      <h4>Change required</h4>
      {{range .Changes}}<div class="change-row"><span class="change-field">{{.Field}}</span><span>{{.Current}}</span><span class="change-arrow">&rarr;</span><span>{{.Required}}</span></div>{{end}}
    </div>
    {{end}}
    {{if .Diff}}
    <div class="remediation-panel">
      <h4>Suggested diff</h4>
      <pre id="{{$.ElementID}}-diff">{{.Diff}}</pre>
      <button type="button" class="copy-btn" data-copy-target="{{$.ElementID}}-diff">Copy diff</button>
    </div>
    {{end}}
    {{if .SafeFix}}
    <div class="remediation-panel">
      <h4>Safe fix</h4>
      {{if .SafeFix.Steps}}<ul>{{range .SafeFix.Steps}}<li>{{.}}</li>{{end}}</ul>{{end}}
      {{if .SafeFix.Command}}<pre id="{{$.ElementID}}-safefix">{{.SafeFix.Command}}</pre><button type="button" class="copy-btn" data-copy-target="{{$.ElementID}}-safefix">Copy command</button>{{end}}
    </div>
    {{end}}
    {{if .Emergency}}
    <div class="remediation-panel emergency-panel">
      <h4>Emergency workaround</h4>
      {{if .Emergency.Steps}}<ul>{{range .Emergency.Steps}}<li>{{.}}</li>{{end}}</ul>{{end}}
      {{if .Emergency.Command}}<pre id="{{$.ElementID}}-emergency">{{.Emergency.Command}}</pre><button type="button" class="copy-btn" data-copy-target="{{$.ElementID}}-emergency">Copy command</button>{{end}}
    </div>
    {{end}}
    {{if .BreakGlass}}
    <div class="remediation-panel breakglass-panel">
      <h4>Break-glass</h4>
      {{if .BreakGlass.Steps}}<ul>{{range .BreakGlass.Steps}}<li>{{.}}</li>{{end}}</ul>{{end}}
      {{if .BreakGlass.Command}}<pre id="{{$.ElementID}}-breakglass">{{.BreakGlass.Command}}</pre><button type="button" class="copy-btn" data-copy-target="{{$.ElementID}}-breakglass">Copy command</button>{{end}}
    </div>
    {{end}}
    {{if .VerifyCommand}}
    <div class="remediation-panel">
      <h4>Verify</h4>
      <pre id="{{$.ElementID}}-verify">{{.VerifyCommand}}</pre>
      <button type="button" class="copy-btn" data-copy-target="{{$.ElementID}}-verify">Copy verify command</button>
    </div>
    {{if .ExpectedResult}}<p class="expected-result">Expected: {{.ExpectedResult}}</p>{{end}}
    {{end}}
    {{end}}
    {{end}}

    {{if .BlockerFindings}}
    <h2 class="section-title">Blockers ({{len .BlockerFindings}})</h2>
    {{range .BlockerFindings}}
    <details class="finding-row blocker" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
      <summary>
        <span class="rule-id">{{.RuleID}}</span>
        <span class="severity-pill blocker">{{.Severity}}</span>
        <span class="confidence-pill">{{.Confidence}}</span>
        {{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}
        {{if .GlobalBlocker}}<span class="global-blocker-badge">GLOBAL API WRITE BLOCKER</span>{{end}}
        <strong class="finding-resource">{{.ResourceLabel}}</strong>
        <span class="finding-message">{{.Message}}</span>
      </summary>
      <div class="finding-body">
        {{if .Evidence}}<h4>Evidence</h4><ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{end}}
        {{if .Remediation}}<div class="remediation-panel"><h4>Remediation</h4><pre id="{{.ElementID}}-remediation">{{.Remediation}}</pre><button type="button" class="copy-btn" data-copy-target="{{.ElementID}}-remediation">Copy remediation</button></div>{{end}}
        {{if .DependencyWarning}}<p class="dependency-warning">This command may fail until the admission webhook blocker is fixed.</p>{{end}}
        {{template "remediationDetail" .}}
      </div>
    </details>
    {{end}}
    {{end}}

    {{if .WarningFindings}}
    <h2 class="section-title">Warnings ({{len .WarningFindings}})</h2>
    {{range .WarningFindings}}
    <details class="finding-row warning" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
      <summary>
        <span class="rule-id">{{.RuleID}}</span>
        <span class="severity-pill warning">{{.Severity}}</span>
        <span class="confidence-pill">{{.Confidence}}</span>
        {{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}
        {{if .GlobalBlocker}}<span class="global-blocker-badge">GLOBAL API WRITE BLOCKER</span>{{end}}
        <strong class="finding-resource">{{.ResourceLabel}}</strong>
        <span class="finding-message">{{.Message}}</span>
      </summary>
      <div class="finding-body">
        {{if .Evidence}}<h4>Evidence</h4><ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{end}}
        {{if .Remediation}}<div class="remediation-panel"><h4>Remediation</h4><pre id="{{.ElementID}}-remediation">{{.Remediation}}</pre><button type="button" class="copy-btn" data-copy-target="{{.ElementID}}-remediation">Copy remediation</button></div>{{end}}
        {{if .DependencyWarning}}<p class="dependency-warning">This command may fail until the admission webhook blocker is fixed.</p>{{end}}
        {{template "remediationDetail" .}}
      </div>
    </details>
    {{end}}
    {{end}}
  </div>

  <div class="tab-panel hidden" data-panel="actions" id="next-actions">
    {{if .NextActions}}
    <h2 class="section-title">Next Actions ({{len .NextActions}})</h2>
    <ol class="next-actions">
    {{range .NextActions}}
      <li class="{{.SeverityClass}}" data-severity="{{.Severity}}" data-rule-ids="{{.RuleIDsJoined}}" data-resource="{{.ResourceLabel}}">
        <div class="action-head">
          <span class="severity-pill {{.SeverityClass}}">{{.Severity}}</span>
          <span class="rule-id">{{.RuleIDsJoined}}</span>
          <strong class="next-action-heading">{{.ResourceLabel}}</strong>
        </div>
        <div class="remediation-panel">
          <h4>Recommended fix</h4>
          <pre id="{{.ElementID}}-remediation">{{.Remediation}}</pre>
          <button type="button" class="copy-btn" data-copy-target="{{.ElementID}}-remediation">Copy remediation</button>
        </div>
        {{if .GroupedPlan}}
        <ol class="grouped-plan">
          {{range .GroupedPlan}}<li>{{.}}</li>{{end}}
        </ol>
        {{end}}
        {{range .Related}}
        <div class="also-see">Also see {{.RuleID}}: {{.Note}}</div>
        {{end}}
      </li>
    {{end}}
    </ol>
    {{end}}
  </div>

  <div class="tab-panel hidden" data-panel="evidence" id="evidence-appendix">
    {{if .AllFindings}}
    <h2 class="section-title">Evidence Appendix</h2>
    <p>Every finding's raw identity data, unmerged — cross-reference by fingerprint for waivers/dedup.</p>
    <div class="table-wrap">
    <table class="appendix">
      <tr><th>Rule ID</th><th>Severity</th><th>Confidence</th><th>Resource</th><th>Fingerprint</th></tr>
      {{range .AllFindings}}
      <tr data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
        <td>{{.RuleID}}</td><td>{{.Severity}}</td><td>{{.Confidence}}</td><td>{{.ResourceLabel}}</td><td class="fingerprint">{{.Fingerprint}}</td>
      </tr>
      {{end}}
    </table>
    </div>
    {{end}}
  </div>

  <footer>Generated by KubePreflight · read-only scan, no cluster or AWS writes.</footer>

  <script>
  (function() {
    var tabButtons = document.querySelectorAll('.tab-button');
    var tabPanels = document.querySelectorAll('.tab-panel');

    function activateTab(name) {
      tabButtons.forEach(function(btn) { btn.classList.toggle('tab-active', btn.getAttribute('data-tab') === name); });
      tabPanels.forEach(function(panel) { panel.classList.toggle('hidden', panel.getAttribute('data-panel') !== name); });
    }

    tabButtons.forEach(function(btn) {
      btn.addEventListener('click', function() { activateTab(btn.getAttribute('data-tab')); });
    });
    document.querySelectorAll('[data-goto-tab]').forEach(function(link) {
      link.addEventListener('click', function(event) {
        event.preventDefault();
        activateTab(link.getAttribute('data-goto-tab'));
      });
    });

    // Printing a tabbed screen view makes no sense on paper — expand every
    // panel and every collapsed finding row before the print dialog opens,
    // then restore the compact on-screen state afterward.
    var reopenedPanels = [];
    window.addEventListener('beforeprint', function() {
      reopenedPanels = [];
      tabPanels.forEach(function(panel) {
        if (panel.classList.contains('hidden')) {
          panel.classList.remove('hidden');
          reopenedPanels.push(panel);
        }
      });
      document.querySelectorAll('.finding-row:not([open])').forEach(function(el) { el.setAttribute('open', ''); el.dataset.reopenedForPrint = 'true'; });
    });
    window.addEventListener('afterprint', function() {
      reopenedPanels.forEach(function(panel) { panel.classList.add('hidden'); });
      reopenedPanels = [];
      document.querySelectorAll('[data-reopened-for-print]').forEach(function(el) { el.removeAttribute('open'); delete el.dataset.reopenedForPrint; });
    });

    var sevBoxes = document.querySelectorAll('.sev-filter');
    var ruleInput = document.getElementById('rule-filter');
    var resourceInput = document.getElementById('resource-filter');
    var countEl = document.getElementById('filter-count');
    var allRows = document.querySelectorAll('[data-severity]');
    var findingRows = document.querySelectorAll('[data-finding]');

    function apply() {
      var activeSevs = {};
      sevBoxes.forEach(function(b) { if (b.checked) { activeSevs[b.value] = true; } });
      var ruleQuery = ruleInput.value.trim().toLowerCase();
      var resourceQuery = resourceInput.value.trim().toLowerCase();

      function matches(row) {
        var sev = row.getAttribute('data-severity');
        var ruleIds = (row.getAttribute('data-rule-ids') || '').toLowerCase();
        var resource = (row.getAttribute('data-resource') || '').toLowerCase();
        return activeSevs[sev] === true &&
          (ruleQuery === '' || ruleIds.indexOf(ruleQuery) !== -1) &&
          (resourceQuery === '' || resource.indexOf(resourceQuery) !== -1);
      }

      allRows.forEach(function(row) { row.classList.toggle('hidden', !matches(row)); });

      // Findings can appear in Blockers/Warnings, Next Actions (merged),
      // and the Evidence Appendix at once — counting every [data-severity]
      // element would triple/quadruple-count the same finding. The visible
      // count is scored only against the Blockers/Warnings finding rows,
      // which are exactly 1:1 with the Summary's blocker/warning totals.
      var shown = 0;
      findingRows.forEach(function(row) { if (matches(row)) { shown++; } });
      countEl.textContent = 'Showing ' + shown + ' of ' + findingRows.length + ' findings';
    }

    sevBoxes.forEach(function(b) { b.addEventListener('change', apply); });
    ruleInput.addEventListener('input', apply);
    resourceInput.addEventListener('input', apply);
    apply();

    document.querySelectorAll('.copy-btn').forEach(function(btn) {
      var originalLabel = btn.textContent;
      btn.addEventListener('click', function(event) {
        event.preventDefault();
        var targetId = btn.getAttribute('data-copy-target');
        var pre = targetId ? document.getElementById(targetId) : btn.previousElementSibling;
        var text = pre ? pre.textContent : '';
        var reset = function() { setTimeout(function() { btn.textContent = originalLabel; }, 1500); };
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text).then(function() { btn.textContent = 'Copied'; reset(); }, function() { btn.textContent = 'Copy unavailable'; reset(); });
        } else {
          btn.textContent = 'Copy unavailable';
          reset();
        }
      });
    });
  })();
  </script>
</body>
</html>
`

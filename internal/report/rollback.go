package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"kubepreflight/internal/rollback"
)

func WriteRollbackJSON(a *rollback.Assessment, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(a)
}

func WriteRollbackTerminal(a *rollback.Assessment, w io.Writer) error {
	var sb strings.Builder
	writeRollbackSummary(&sb, a, "")
	writeRollbackChecks(&sb, a, "  - ")
	_, err := w.Write([]byte(sb.String()))
	return err
}

func WriteRollbackMarkdown(a *rollback.Assessment, w io.Writer) error {
	var sb strings.Builder
	fmt.Fprintln(&sb, "# KubePreflight Rollback Readiness")
	fmt.Fprintln(&sb)
	writeRollbackSummary(&sb, a, "markdown")
	if len(a.Recommendation.ReasonCodes) > 0 {
		fmt.Fprintln(&sb, "## Reason Codes")
		fmt.Fprintln(&sb)
		for _, reason := range a.Recommendation.ReasonCodes {
			fmt.Fprintf(&sb, "- `%s`\n", reason)
		}
		fmt.Fprintln(&sb)
	}
	if len(a.Checks) > 0 {
		fmt.Fprintln(&sb, "## Checks")
		fmt.Fprintln(&sb)
		fmt.Fprintln(&sb, "| Check | Status | Reason codes | Evidence |")
		fmt.Fprintln(&sb, "|---|---|---|---|")
		for _, check := range a.Checks {
			fmt.Fprintf(&sb, "| %s | `%s` | %s | %s |\n",
				check.Title,
				check.Status,
				rollbackReasonList(check.ReasonCodes),
				markdownEvidence(check.Evidence),
			)
		}
		fmt.Fprintln(&sb)
	}
	_, err := w.Write([]byte(sb.String()))
	return err
}

func WriteRollbackHTML(a *rollback.Assessment, w io.Writer) error {
	return rollbackHTMLTmpl.Execute(w, rollbackHTMLData{
		Assessment: *a,
		Generated:  formatRollbackTime(a.GeneratedAt),
		Reasons:    rollbackReasonList(a.Recommendation.ReasonCodes),
		Evidence:   evidenceLabel(a.Evidence.Complete),
		Window:     rollbackWindowLabel(a),
		Decision:   string(a.Recommendation.Decision),
		DecisionUI: strings.ReplaceAll(string(a.Recommendation.Decision), "_", " "),
	})
}

func writeRollbackSummary(sb *strings.Builder, a *rollback.Assessment, mode string) {
	if mode == "markdown" {
		fmt.Fprintln(sb, "| | |")
		fmt.Fprintln(sb, "|---|---|")
		fmt.Fprintf(sb, "| **Schema** | `%s` |\n", a.SchemaVersion)
		fmt.Fprintf(sb, "| **Mode** | `%s` |\n", a.Mode)
		fmt.Fprintf(sb, "| **Cluster** | %s |\n", orDash(a.Cluster.Name))
		fmt.Fprintf(sb, "| **Region** | %s |\n", orDash(a.Cluster.Region))
		fmt.Fprintf(sb, "| **Current version** | %s |\n", orDash(a.Cluster.CurrentVersion))
		fmt.Fprintf(sb, "| **Rollback target** | %s |\n", orDash(a.Cluster.RollbackTargetVersion))
		fmt.Fprintf(sb, "| **Eligibility** | `%s` |\n", a.Eligibility.Status)
		fmt.Fprintf(sb, "| **Readiness** | `%s` |\n", a.Readiness.Status)
		fmt.Fprintf(sb, "| **Recommendation** | `%s` |\n", a.Recommendation.Decision)
		fmt.Fprintf(sb, "| **Confidence** | `%s` |\n", a.Recommendation.Confidence)
		fmt.Fprintf(sb, "| **Evidence complete** | %s |\n", yesNo(a.Evidence.Complete))
		fmt.Fprintf(sb, "| **Rollback window** | %s |\n\n", rollbackWindowLabel(a))
		return
	}

	fmt.Fprintf(sb, "KubePreflight rollback readiness — cluster: %s  current: %s  target: %s\n",
		orDash(a.Cluster.Name), orDash(a.Cluster.CurrentVersion), orDash(a.Cluster.RollbackTargetVersion))
	fmt.Fprintf(sb, "Eligibility: %s\n", a.Eligibility.Status)
	fmt.Fprintf(sb, "Readiness: %s  blockers: %d  warnings: %d  unknowns: %d\n",
		a.Readiness.Status, a.Readiness.Blockers, a.Readiness.Warnings, a.Readiness.Unknowns)
	fmt.Fprintf(sb, "Recommendation: %s  confidence: %s\n", a.Recommendation.Decision, a.Recommendation.Confidence)
	fmt.Fprintf(sb, "Evidence complete: %s\n", yesNo(a.Evidence.Complete))
	if window := rollbackWindowLabel(a); window != "unknown" {
		fmt.Fprintf(sb, "Rollback window: %s\n", window)
	}
	if len(a.Recommendation.ReasonCodes) > 0 {
		fmt.Fprintf(sb, "Reason codes: %s\n", rollbackReasonList(a.Recommendation.ReasonCodes))
	}
	fmt.Fprintln(sb)
}

func writeRollbackChecks(sb *strings.Builder, a *rollback.Assessment, prefix string) {
	if len(a.Checks) == 0 {
		return
	}
	fmt.Fprintln(sb, "Checks")
	for _, check := range a.Checks {
		fmt.Fprintf(sb, "%s[%s] %s", prefix, check.Status, check.Title)
		if len(check.ReasonCodes) > 0 {
			fmt.Fprintf(sb, " (%s)", rollbackReasonList(check.ReasonCodes))
		}
		fmt.Fprintln(sb)
		for _, evidence := range check.Evidence {
			fmt.Fprintf(sb, "%s  evidence: %s\n", prefix, evidence)
		}
	}
}

func rollbackReasonList(reasons []rollback.ReasonCode) string {
	if len(reasons) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		parts = append(parts, string(reason))
	}
	return strings.Join(parts, ", ")
}

func markdownEvidence(evidence []string) string {
	if len(evidence) == 0 {
		return "none"
	}
	return strings.ReplaceAll(strings.Join(evidence, "<br>"), "|", "\\|")
}

func rollbackWindowLabel(a *rollback.Assessment) string {
	if a.Eligibility.RemainingMinutes == nil {
		return "unknown"
	}
	minutes := *a.Eligibility.RemainingMinutes
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("at least %dh %dm remaining", minutes/60, minutes%60)
}

func evidenceLabel(complete bool) string {
	if complete {
		return "complete"
	}
	return "incomplete"
}

func formatRollbackTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format(time.RFC3339)
}

type rollbackHTMLData struct {
	Assessment rollback.Assessment
	Generated  string
	Reasons    string
	Evidence   string
	Window     string
	Decision   string
	DecisionUI string
}

var rollbackHTMLTmpl = template.Must(template.New("rollback").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>KubePreflight Rollback Readiness</title>
  <style>
    body { margin: 0; font: 14px/1.5 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #24211d; background: #f6f4ee; }
    main { max-width: 1100px; margin: 0 auto; padding: 28px 20px 48px; }
    h1 { margin: 0 0 8px; font: 500 32px Georgia, serif; }
    h2 { margin-top: 28px; font: 500 22px Georgia, serif; }
    .meta, .checks { display: grid; gap: 10px; }
    .meta { grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); margin: 22px 0; }
    .item, .check { border: 1px solid #ddd8ca; background: #fffdf8; border-radius: 8px; padding: 14px 16px; }
    .label { display: block; color: #6d665c; font-size: 12px; text-transform: uppercase; letter-spacing: .04em; }
    .value { display: block; margin-top: 3px; font-weight: 700; overflow-wrap: anywhere; }
    .decision { border-left: 5px solid #315c85; }
    .decision.do_not_proceed { border-left-color: #b94b40; }
    .decision.fix_forward_preferred, .decision.operator_decision_required { border-left-color: #c08a24; }
    code { background: #eee8db; padding: 2px 5px; border-radius: 4px; }
    ul { margin: 8px 0 0; padding-left: 20px; }
  </style>
</head>
<body>
<main>
  <h1>KubePreflight Rollback Readiness</h1>
  <p>Generated {{.Generated}}. Assessment only; KubePreflight does not execute rollback or mutate cluster resources.</p>
  <section class="meta">
    <div class="item"><span class="label">Cluster</span><span class="value">{{or .Assessment.Cluster.Name "unknown"}}</span></div>
    <div class="item"><span class="label">Region</span><span class="value">{{or .Assessment.Cluster.Region "unknown"}}</span></div>
    <div class="item"><span class="label">Current version</span><span class="value">{{or .Assessment.Cluster.CurrentVersion "unknown"}}</span></div>
    <div class="item"><span class="label">Rollback target</span><span class="value">{{or .Assessment.Cluster.RollbackTargetVersion "unknown"}}</span></div>
    <div class="item"><span class="label">Eligibility</span><span class="value">{{.Assessment.Eligibility.Status}}</span></div>
    <div class="item"><span class="label">Readiness</span><span class="value">{{.Assessment.Readiness.Status}}</span></div>
    <div class="item decision {{.Decision}}"><span class="label">Recommendation</span><span class="value">{{.DecisionUI}}</span></div>
    <div class="item"><span class="label">Confidence</span><span class="value">{{.Assessment.Recommendation.Confidence}}</span></div>
    <div class="item"><span class="label">Evidence</span><span class="value">{{.Evidence}}</span></div>
    <div class="item"><span class="label">Rollback window</span><span class="value">{{.Window}}</span></div>
  </section>
  <h2>Reason Codes</h2>
  <p><code>{{.Reasons}}</code></p>
  <h2>Checks</h2>
  <section class="checks">
    {{range .Assessment.Checks}}
    <article class="check">
      <strong>{{.Title}}</strong>
      <p>Status: <code>{{.Status}}</code></p>
      {{if .ReasonCodes}}<p>Reasons: {{range .ReasonCodes}}<code>{{.}}</code> {{end}}</p>{{end}}
      {{if .Evidence}}<ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{end}}
    </article>
    {{end}}
  </section>
</main>
</body>
</html>`))

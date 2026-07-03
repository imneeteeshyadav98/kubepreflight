package report

import (
	"html/template"
	"io"
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
}

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
}

type htmlViewData struct {
	Cluster            string
	Target             string
	Provider           string
	NamespaceAllowlist string
	ScannedAt          string
	Result             string
	ResultClass        string
	Blockers           int
	Warnings           int
	Infos              int
	Assumptions        []string
	BlockerFindings    []htmlFinding
	WarningFindings    []htmlFinding
	NextActions        []htmlNextAction
	AllFindings        []htmlFinding
}

// WriteHTML renders the same Report data as WriteTerminal — identical
// grouping and Next Actions dedup (view.go) — as a standalone HTML file:
// inline CSS and a small vanilla-JS filter/search pass, no external
// assets, no build step, no CDN dependency. Still a single self-contained
// file: works as a CAB-ticket attachment or an offline double-click open
// with no internet access needed to view or interact with it.
func WriteHTML(r *findings.Report, w io.Writer) error {
	providerLabel := r.Provider
	if providerLabel == "" {
		providerLabel = "cluster-only"
	}

	blockers := filterAndSort(r.Findings, findings.SeverityBlocker)
	warnings := filterAndSort(r.Findings, findings.SeverityWarning)

	actionable := make([]findings.Finding, 0, len(blockers)+len(warnings))
	actionable = append(actionable, blockers...)
	actionable = append(actionable, warnings...)

	data := htmlViewData{
		Cluster:            orDash(r.ClusterContext),
		Target:             r.TargetVersion,
		Provider:           providerLabel,
		NamespaceAllowlist: strings.Join(r.NamespaceAllowlist, ", "),
		ScannedAt:          r.ScannedAt.Format("2006-01-02 15:04:05 MST"),
		Result:             r.Result(),
		ResultClass:        resultClass(r.Result()),
		Blockers:           r.Summary.Blockers,
		Warnings:           r.Summary.Warnings,
		Infos:              r.Summary.Infos,
		Assumptions:        r.Assumptions,
		BlockerFindings:    toHTMLFindings(blockers),
		WarningFindings:    toHTMLFindings(warnings),
		NextActions:        toHTMLNextActions(buildNextActions(actionable)),
		AllFindings:        toHTMLFindings(allSorted(r.Findings)),
	}

	return htmlTmpl.Execute(w, data)
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

func toHTMLFindings(fs []findings.Finding) []htmlFinding {
	out := make([]htmlFinding, len(fs))
	for i, f := range fs {
		out[i] = htmlFinding{Finding: f, ResourceLabel: findingResourceLabel(f)}
	}
	return out
}

func toHTMLNextActions(actions []NextAction) []htmlNextAction {
	out := make([]htmlNextAction, len(actions))
	for i, a := range actions {
		var related []htmlRelatedNote
		for _, f := range a.Related {
			related = append(related, htmlRelatedNote{RuleID: f.RuleID, Note: firstLine(f.Remediation)})
		}
		out[i] = htmlNextAction{
			ResourceLabel: a.ResourceLabel,
			RuleIDsJoined: strings.Join(a.RuleIDs, ", "),
			Severity:      a.Severity,
			Remediation:   a.Primary.Remediation,
			Related:       related,
		}
	}
	return out
}

const htmlTemplateSource = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>KubePreflight Scan Report — {{.Cluster}}</title>
<style>
  :root {
    --blocked: #b3261e;
    --warn: #7a5900;
    --clean: #1b6e3c;
    --bg: #ffffff;
    --card-bg: #f7f7f8;
    --border: #dcdde0;
    --text: #1a1a1a;
    --muted: #5b5f66;
  }
  * { box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    color: var(--text);
    background: var(--bg);
    max-width: 900px;
    margin: 0 auto;
    padding: 32px 20px 80px;
    line-height: 1.5;
  }
  h1 { font-size: 22px; margin-bottom: 4px; }
  h2 { font-size: 18px; margin-top: 40px; border-bottom: 1px solid var(--border); padding-bottom: 6px; }
  h3 { font-size: 15px; margin-bottom: 4px; }
  .summary-table { border-collapse: collapse; margin: 16px 0 8px; }
  .summary-table td { padding: 4px 12px 4px 0; vertical-align: top; }
  .summary-table td.label { color: var(--muted); white-space: nowrap; }
  .badge {
    display: inline-block;
    padding: 3px 10px;
    border-radius: 4px;
    font-weight: 600;
    font-size: 13px;
    color: #fff;
  }
  .badge.blocked { background: var(--blocked); }
  .badge.warn { background: var(--warn); }
  .badge.clean { background: var(--clean); }
  .toolbar {
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 12px 16px;
    margin: 20px 0;
    background: var(--card-bg);
    position: sticky;
    top: 0;
    z-index: 10;
  }
  .toolbar-row { display: flex; flex-wrap: wrap; gap: 14px; align-items: center; margin-bottom: 8px; }
  .toolbar-row:last-of-type { margin-bottom: 0; }
  .toolbar-label { font-weight: 600; font-size: 13px; color: var(--muted); }
  .toolbar label { font-size: 13px; display: inline-flex; align-items: center; gap: 4px; cursor: pointer; }
  .toolbar input[type="text"] {
    padding: 5px 8px;
    border: 1px solid var(--border);
    border-radius: 4px;
    font-size: 13px;
    flex: 1;
    min-width: 160px;
  }
  .toolbar-count { font-size: 12px; color: var(--muted); margin-top: 6px; }
  .hidden { display: none !important; }
  .finding {
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--card-bg);
    padding: 14px 16px;
    margin-bottom: 12px;
  }
  .finding .rule-id {
    display: inline-block;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 12px;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 4px;
    background: #e8e8ea;
    margin-right: 6px;
  }
  .finding.blocker .rule-id { background: #fdeceb; color: var(--blocked); }
  .finding.warning .rule-id { background: #fdf6e3; color: var(--warn); }
  .finding .confidence { color: var(--muted); font-size: 12px; }
  .finding ul { margin: 6px 0; padding-left: 20px; }
  details { margin: 8px 0 0; }
  details summary {
    cursor: pointer;
    font-weight: 600;
    font-size: 12.5px;
    color: var(--muted);
    padding: 2px 0;
    list-style: revert;
  }
  details summary:hover { color: var(--text); }
  details[open] summary { margin-bottom: 4px; }
  pre {
    background: #f0f0f2;
    border: 1px solid var(--border);
    border-radius: 4px;
    padding: 10px 12px;
    overflow-x: auto;
    font-size: 12.5px;
    white-space: pre-wrap;
    word-break: break-word;
  }
  ol.next-actions > li { margin-bottom: 18px; }
  .also-see { color: var(--muted); font-size: 13px; margin-top: 6px; }
  table.appendix { border-collapse: collapse; width: 100%; font-size: 13px; }
  table.appendix th, table.appendix td { border: 1px solid var(--border); padding: 6px 8px; text-align: left; }
  table.appendix th { background: var(--card-bg); }
  table.appendix td.fingerprint { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 11px; word-break: break-all; }
  footer { margin-top: 60px; color: var(--muted); font-size: 12px; border-top: 1px solid var(--border); padding-top: 12px; }
</style>
</head>
<body>
  <h1>KubePreflight Scan Report</h1>
  <table class="summary-table">
    <tr><td class="label">Cluster</td><td>{{.Cluster}}</td></tr>
    <tr><td class="label">Target version</td><td>{{.Target}}</td></tr>
    <tr><td class="label">Provider</td><td>{{.Provider}}</td></tr>
    {{if .NamespaceAllowlist}}<tr><td class="label">Namespace allowlist</td><td>{{.NamespaceAllowlist}}</td></tr>{{end}}
    <tr><td class="label">Scanned at</td><td>{{.ScannedAt}}</td></tr>
    <tr><td class="label">Result</td><td><span class="badge {{.ResultClass}}">{{.Result}}</span></td></tr>
    <tr><td class="label">Summary</td><td>{{.Blockers}} blocker(s), {{.Warnings}} warning(s), {{.Infos}} info(s)</td></tr>
  </table>

  {{range .Assumptions}}<p><strong>Assumption:</strong> {{.}}</p>{{end}}

  <div class="toolbar">
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

  {{if .BlockerFindings}}
  <h2>Blockers ({{len .BlockerFindings}})</h2>
  {{range .BlockerFindings}}
  <div class="finding blocker" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
    <h3><span class="rule-id">{{.RuleID}}</span>{{.Message}}</h3>
    <div class="confidence">Confidence: {{.Confidence}}</div>
    {{if .Evidence}}
    <details><summary>Evidence ({{len .Evidence}})</summary>
    <ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    </details>
    {{end}}
    {{if .Remediation}}
    <details><summary>Remediation</summary>
    <pre>{{.Remediation}}</pre>
    </details>
    {{end}}
  </div>
  {{end}}
  {{end}}

  {{if .WarningFindings}}
  <h2>Warnings ({{len .WarningFindings}})</h2>
  {{range .WarningFindings}}
  <div class="finding warning" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
    <h3><span class="rule-id">{{.RuleID}}</span>{{.Message}}</h3>
    <div class="confidence">Confidence: {{.Confidence}}</div>
    {{if .Evidence}}
    <details><summary>Evidence ({{len .Evidence}})</summary>
    <ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    </details>
    {{end}}
    {{if .Remediation}}
    <details><summary>Remediation</summary>
    <pre>{{.Remediation}}</pre>
    </details>
    {{end}}
  </div>
  {{end}}
  {{end}}

  {{if .NextActions}}
  <h2>Next Actions ({{len .NextActions}})</h2>
  <ol class="next-actions">
  {{range .NextActions}}
    <li data-severity="{{.Severity}}" data-rule-ids="{{.RuleIDsJoined}}" data-resource="{{.ResourceLabel}}">
      <strong>[{{.Severity}}] {{.ResourceLabel}}</strong> ({{.RuleIDsJoined}})
      <pre>{{.Remediation}}</pre>
      {{range .Related}}
      <div class="also-see">Also see {{.RuleID}}: {{.Note}}</div>
      {{end}}
    </li>
  {{end}}
  </ol>
  {{end}}

  {{if .AllFindings}}
  <h2>Evidence Appendix</h2>
  <p>Every finding's raw identity data, unmerged — cross-reference by fingerprint for waivers/dedup.</p>
  <table class="appendix">
    <tr><th>Rule ID</th><th>Severity</th><th>Confidence</th><th>Resource</th><th>Fingerprint</th></tr>
    {{range .AllFindings}}
    <tr data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
      <td>{{.RuleID}}</td><td>{{.Severity}}</td><td>{{.Confidence}}</td><td>{{.ResourceLabel}}</td><td class="fingerprint">{{.Fingerprint}}</td>
    </tr>
    {{end}}
  </table>
  {{end}}

  <footer>Generated by KubePreflight · read-only scan, no cluster or AWS writes.</footer>

  <script>
  (function() {
    var sevBoxes = document.querySelectorAll('.sev-filter');
    var ruleInput = document.getElementById('rule-filter');
    var resourceInput = document.getElementById('resource-filter');
    var countEl = document.getElementById('filter-count');
    var rows = document.querySelectorAll('[data-severity]');

    function apply() {
      var activeSevs = {};
      sevBoxes.forEach(function(b) { if (b.checked) { activeSevs[b.value] = true; } });
      var ruleQuery = ruleInput.value.trim().toLowerCase();
      var resourceQuery = resourceInput.value.trim().toLowerCase();

      var shown = 0;
      rows.forEach(function(row) {
        var sev = row.getAttribute('data-severity');
        var ruleIds = (row.getAttribute('data-rule-ids') || '').toLowerCase();
        var resource = (row.getAttribute('data-resource') || '').toLowerCase();

        var match = activeSevs[sev] === true &&
          (ruleQuery === '' || ruleIds.indexOf(ruleQuery) !== -1) &&
          (resourceQuery === '' || resource.indexOf(resourceQuery) !== -1);

        row.classList.toggle('hidden', !match);
        if (match) { shown++; }
      });
      countEl.textContent = 'Showing ' + shown + ' of ' + rows.length + ' findings';
    }

    sevBoxes.forEach(function(b) { b.addEventListener('change', apply); });
    ruleInput.addEventListener('input', apply);
    resourceInput.addEventListener('input', apply);
    apply();
  })();
  </script>
</body>
</html>
`

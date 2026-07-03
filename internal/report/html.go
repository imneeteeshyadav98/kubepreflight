package report

import (
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

type htmlConfidenceStat struct {
	Tier  findings.ConfidenceTier
	Count int
}

type htmlViewData struct {
	Cluster            string
	Target             string
	Provider           string
	AWSEnrichment      bool
	NamespaceAllowlist string
	ScannedAt          string
	Result             string
	ResultClass        string
	Blockers           int
	Warnings           int
	Infos              int
	TotalFindings      int
	Assumptions        []string
	ConfidenceMix      []htmlConfidenceStat
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
// with no internet access needed to view or interact with it. The visual
// language (navy banner, eyebrow labels, metric cards, severity/confidence
// pills) intentionally mirrors the local Console (web/) so the CAB-style
// static report and the interactive viewer read as the same product.
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
		AWSEnrichment:      awsEnrichment(r),
		NamespaceAllowlist: strings.Join(r.NamespaceAllowlist, ", "),
		ScannedAt:          r.ScannedAt.Format("2006-01-02 15:04:05 MST"),
		Result:             r.Result(),
		ResultClass:        resultClass(r.Result()),
		Blockers:           r.Summary.Blockers,
		Warnings:           r.Summary.Warnings,
		Infos:              r.Summary.Infos,
		TotalFindings:      len(r.Findings),
		Assumptions:        r.Assumptions,
		ConfidenceMix:      confidenceMix(r.Findings),
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

func toHTMLFindings(fs []findings.Finding) []htmlFinding {
	out := make([]htmlFinding, len(fs))
	for i, f := range fs {
		out[i] = htmlFinding{Finding: f, ResourceLabel: findingResourceLabel(f), PlaneLabel: planeLabel(f)}
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
  }
  * { box-sizing: border-box; }
  body {
    font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    color: var(--ink);
    background: var(--paper);
    max-width: 980px;
    margin: 0 auto;
    padding: 0 20px 80px;
    line-height: 1.55;
  }
  code, pre, .eyebrow, .badge, .severity-pill, .confidence-pill, .rule-id { font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace; }
  .eyebrow { margin: 0; color: var(--blue); font-size: 10px; font-weight: 700; letter-spacing: .14em; text-transform: uppercase; }
  h1 { margin: 6px 0 0; font: 500 clamp(24px, 4vw, 34px)/1.15 Georgia, "Times New Roman", serif; letter-spacing: -.03em; }
  h2.section-title { margin: 40px 0 14px; font: 500 22px Georgia, serif; border-bottom: 1px solid var(--line); padding-bottom: 8px; }
  h3 { font-size: 15px; margin-bottom: 4px; }

  .banner { margin-top: 24px; padding: 26px 30px; background: var(--navy); color: white; box-shadow: var(--shadow); }
  .banner .eyebrow { color: var(--mint); }
  .banner-result { display: flex; align-items: center; flex-wrap: wrap; gap: 14px; margin-top: 6px; }
  .banner-result h1 { color: white; }
  .banner-meta { display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 16px 28px; margin: 22px 0 0; padding-top: 18px; border-top: 1px solid rgba(255,255,255,.14); }
  .banner-meta dt { color: #8ca49e; font-size: 10px; text-transform: uppercase; letter-spacing: .1em; }
  .banner-meta dd { margin: 4px 0 0; font: 12px monospace; }

  .badge { display: inline-block; padding: 7px 10px; border: 1px solid currentColor; font-size: 11px; font-weight: 700; letter-spacing: .08em; }
  .badge.blocked { color: #ffaaa1; } .badge.warn { color: #ffd28c; } .badge.clean { color: var(--mint); }

  .summary-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-top: 16px; }
  .metric { min-height: 110px; padding: 18px; border: 1px solid var(--line); background: var(--surface); }
  .metric span { display: block; color: var(--muted); font-size: 11px; }
  .metric strong { display: block; margin: 14px 0 4px; font-size: 28px; }
  .metric-blocker strong { color: var(--red); } .metric-warning strong { color: var(--amber); } .metric-result { border-top: 4px solid var(--navy); }

  .confidence-panel { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px; margin-top: 12px; padding: 18px 22px; border: 1px solid var(--line); background: var(--surface); }
  .confidence-panel .eyebrow { margin-bottom: 4px; }
  .confidence-list { display: flex; flex-wrap: wrap; gap: 8px; }
  .confidence-stat { display: flex; align-items: center; gap: 8px; padding: 7px 10px; border: 1px solid var(--line); font-size: 11px; }
  .confidence-stat b { font: 700 13px monospace; }

  .assumptions { margin-top: 16px; padding: 14px 18px; border-left: 3px solid var(--blue); background: var(--blue-soft); font-size: 12.5px; }
  .assumptions p { margin: 4px 0; }

  .sticky-header { position: sticky; top: 0; z-index: 10; margin-top: 24px; background: var(--paper); padding-top: 10px; }
  .report-nav { display: flex; flex-wrap: wrap; gap: 4px 18px; padding: 10px 4px; border-bottom: 1px solid var(--line); font-size: 12px; }
  .report-nav a { color: var(--blue); text-decoration: none; font-weight: 600; }
  .report-nav a:hover { text-decoration: underline; }

  .toolbar { border: 1px solid var(--line); padding: 12px 16px; margin: 10px 0 0; background: var(--surface); }
  .toolbar-row { display: flex; flex-wrap: wrap; gap: 14px; align-items: center; margin-bottom: 8px; }
  .toolbar-row:last-of-type { margin-bottom: 0; }
  .toolbar-label { font-weight: 600; font-size: 12px; color: var(--muted); }
  .toolbar label { font-size: 12.5px; display: inline-flex; align-items: center; gap: 4px; cursor: pointer; }
  .toolbar input[type="text"] { padding: 6px 8px; border: 1px solid var(--line); font-size: 12.5px; flex: 1; min-width: 160px; background: white; }
  .toolbar-count { font-size: 11.5px; color: var(--muted); margin-top: 6px; }
  .hidden { display: none !important; }

  .finding { border: 1px solid var(--line); background: var(--surface); padding: 16px 18px; margin-bottom: 12px; }
  .finding-badges { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
  .severity-pill, .confidence-pill, .plane-pill, .rule-id { display: inline-flex; align-items: center; white-space: nowrap; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; }
  .severity-pill.blocker { background: var(--red-soft); color: #8e2d25; }
  .severity-pill.warning { background: var(--amber-soft); color: #754706; }
  .severity-pill.info { background: var(--blue-soft); color: var(--blue); }
  .confidence-pill { border: 1px solid var(--line); color: var(--blue); background: white; }
  .plane-pill { gap: 5px; color: var(--muted); background: #f0efe8; }
  .rule-id { background: #eceae0; color: var(--ink); }
  .finding.blocker .rule-id { background: var(--red-soft); color: #8e2d25; }
  .finding.warning .rule-id { background: var(--amber-soft); color: #754706; }
  .finding .confidence-line { color: var(--muted); font-size: 12px; }
  .finding ul { margin: 6px 0; padding-left: 20px; }
  details { margin: 8px 0 0; }
  details summary { cursor: pointer; font-weight: 600; font-size: 12px; color: var(--muted); padding: 2px 0; list-style: revert; }
  details summary:hover { color: var(--ink); }
  details[open] summary { margin-bottom: 4px; }
  pre { background: #f5f4ee; border: 1px solid var(--line); padding: 10px 12px; overflow-x: auto; font-size: 12.5px; white-space: pre-wrap; word-break: break-word; }
  .copy-btn { margin-top: 6px; padding: 5px 10px; border: 1px solid var(--line); background: white; color: var(--blue); font-size: 11px; font-weight: 700; cursor: pointer; }
  .copy-btn:hover { background: var(--blue-soft); }

  ol.next-actions { list-style: none; margin: 0; padding: 0; }
  ol.next-actions > li { border: 1px solid var(--line); background: var(--surface); padding: 16px 18px; margin-bottom: 12px; }
  .also-see { color: var(--muted); font-size: 12.5px; margin-top: 6px; }

  table.appendix { border-collapse: collapse; width: 100%; font-size: 12.5px; background: var(--surface); }
  table.appendix th, table.appendix td { border: 1px solid var(--line); padding: 7px 9px; text-align: left; }
  table.appendix th { background: #f0efe8; font-size: 10.5px; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  table.appendix td.fingerprint { font-family: monospace; font-size: 11px; word-break: break-all; }

  footer { margin-top: 60px; color: var(--muted); font-size: 12px; border-top: 1px solid var(--line); padding-top: 12px; }
</style>
</head>
<body>
  <header class="banner" id="summary">
    <p class="eyebrow">Upgrade readiness report</p>
    <h1>KubePreflight Scan Report</h1>
    <div class="banner-result">
      <span class="badge {{.ResultClass}}">{{.Result}}</span>
    </div>
    <dl class="banner-meta">
      <div><dt>Cluster</dt><dd>{{.Cluster}}</dd></div>
      <div><dt>Target version</dt><dd>{{.Target}}</dd></div>
      <div><dt>Provider</dt><dd>{{.Provider}}</dd></div>
      <div><dt>AWS enrichment</dt><dd>{{.AWSEnrichment}}</dd></div>
      <div><dt>Scanned at</dt><dd>{{.ScannedAt}}</dd></div>
      {{if .NamespaceAllowlist}}<div><dt>Namespace allowlist</dt><dd>{{.NamespaceAllowlist}}</dd></div>{{end}}
    </dl>
  </header>

  <section class="summary-grid" aria-label="Scan summary">
    <article class="metric metric-result"><span>Decision</span><strong>{{.Result}}</strong></article>
    <article class="metric metric-blocker"><span>Blockers</span><strong>{{.Blockers}}</strong></article>
    <article class="metric metric-warning"><span>Warnings</span><strong>{{.Warnings}}</strong></article>
    <article class="metric"><span>Info</span><strong>{{.Infos}}</strong></article>
  </section>

  {{if .ConfidenceMix}}
  <section class="confidence-panel">
    <div><p class="eyebrow">Evidence posture</p></div>
    <div class="confidence-list">
      {{range .ConfidenceMix}}<div class="confidence-stat"><b>{{.Count}}</b><span>{{.Tier}}</span></div>{{end}}
    </div>
  </section>
  {{end}}

  {{if .Assumptions}}
  <section class="assumptions">
    {{range .Assumptions}}<p><strong>Assumption:</strong> {{.}}</p>{{end}}
  </section>
  {{end}}

  <div class="sticky-header">
    <nav class="report-nav" aria-label="Report sections">
      <a href="#summary">Summary</a>
      {{if .BlockerFindings}}<a href="#blockers">Blockers</a>{{end}}
      {{if .WarningFindings}}<a href="#warnings">Warnings</a>{{end}}
      {{if .NextActions}}<a href="#next-actions">Next actions</a>{{end}}
      {{if .AllFindings}}<a href="#evidence-appendix">Evidence appendix</a>{{end}}
    </nav>
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
  </div>

  {{if .BlockerFindings}}
  <h2 class="section-title" id="blockers">Blockers ({{len .BlockerFindings}})</h2>
  {{range .BlockerFindings}}
  <div class="finding blocker" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
    <div class="finding-badges">
      <span class="rule-id">{{.RuleID}}</span>
      <span class="severity-pill blocker">{{.Severity}}</span>
      <span class="confidence-pill">{{.Confidence}}</span>
      {{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}
    </div>
    <h3>{{.ResourceLabel}}</h3>
    <p class="confidence-line">{{.Message}}</p>
    {{if .Evidence}}
    <details><summary>Evidence ({{len .Evidence}})</summary>
    <ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    </details>
    {{end}}
    {{if .Remediation}}
    <details><summary>Remediation</summary>
    <pre>{{.Remediation}}</pre>
    <button type="button" class="copy-btn">Copy remediation</button>
    </details>
    {{end}}
  </div>
  {{end}}
  {{end}}

  {{if .WarningFindings}}
  <h2 class="section-title" id="warnings">Warnings ({{len .WarningFindings}})</h2>
  {{range .WarningFindings}}
  <div class="finding warning" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
    <div class="finding-badges">
      <span class="rule-id">{{.RuleID}}</span>
      <span class="severity-pill warning">{{.Severity}}</span>
      <span class="confidence-pill">{{.Confidence}}</span>
      {{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}
    </div>
    <h3>{{.ResourceLabel}}</h3>
    <p class="confidence-line">{{.Message}}</p>
    {{if .Evidence}}
    <details><summary>Evidence ({{len .Evidence}})</summary>
    <ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>
    </details>
    {{end}}
    {{if .Remediation}}
    <details><summary>Remediation</summary>
    <pre>{{.Remediation}}</pre>
    <button type="button" class="copy-btn">Copy remediation</button>
    </details>
    {{end}}
  </div>
  {{end}}
  {{end}}

  {{if .NextActions}}
  <h2 class="section-title" id="next-actions">Next Actions ({{len .NextActions}})</h2>
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
  <h2 class="section-title" id="evidence-appendix">Evidence Appendix</h2>
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
      // count is scored only against the Blockers/Warnings finding cards,
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
      btn.addEventListener('click', function() {
        var pre = btn.previousElementSibling;
        var text = pre ? pre.textContent : '';
        var reset = function() { setTimeout(function() { btn.textContent = 'Copy remediation'; }, 1500); };
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

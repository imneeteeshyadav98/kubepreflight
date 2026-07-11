package report

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"

	"kubepreflight/internal/findings"
)

// html/template is deliberate, not text/template: remediation text
// can contain raw shell placeholder syntax like `--cluster-name <cluster>`.
// Rendered through text/template or naive string concatenation, browsers
// would interpret <cluster> as an unknown HTML tag and silently
// drop them from the page. html/template's contextual auto-escaping is
// what keeps the rendered report byte-faithful to the actual finding data.
var htmlTmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"severityActionLabel": severityActionLabel,
	"priorityClass":       priorityClass,
	"yesNo":               yesNo,
}).Parse(htmlTemplateSource))

// priorityClass renders a findings.Priority ("P1".."P4") as its lowercase
// CSS class ("p1".."p4") — falls back to "p4" for an empty/unrecognized
// value so a Finding somehow missing Priority never renders an empty
// class attribute.
func priorityClass(priority string) string {
	switch priority {
	case "P1", "P2", "P3", "P4":
		return strings.ToLower(priority)
	default:
		return "p4"
	}
}

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
	// Title is the primary finding's rule-general short label (see
	// ruleCopyByID) — used by the Summary tab's checklist so each item
	// reads as "Fix <title>" instead of a bare rule ID.
	Title       string
	Severity    findings.Severity
	Remediation string
	Related     []htmlRelatedNote
	// ElementID is a per-rendered-instance-unique DOM id base, mirroring
	// htmlFinding.ElementID, for this action's own copy-target <pre>.
	ElementID string
	// GroupedPlan is a synthesized, numbered remediation plan for a merged
	// group (2+ findings sharing a resource) — nil for a single-finding
	// group, where the template falls back to the plain Remediation <pre>.
	GroupedPlan []string
	// Command is the primary finding's SafeFix command, if it has one —
	// surfaced directly in the Summary tab's checklist preview so an
	// operator sees the actual command without switching to the Next
	// Actions tab. Empty when the primary finding has no SafeFix command.
	Command string
	// Priority/PriorityReason are the primary finding's (see
	// findings.AssignPriority) — a merged action inherits its most urgent
	// finding's priority since NextAction.Primary is already picked by
	// highest severity.
	Priority       string
	PriorityReason string
}

// SeverityClass renders the action's severity as the lowercase CSS class
// (blocker/warning/info) used throughout the template's severity styling.
func (a htmlNextAction) SeverityClass() string { return strings.ToLower(string(a.Severity)) }

type htmlConfidenceStat struct {
	Tier  findings.ConfidenceTier
	Count int
}

type htmlCoverageIssue struct {
	Plane  string
	Errors []string
}

type htmlTopRisk struct {
	htmlFinding
	Rank int
	// Title/Why are short, rule-general operator-facing copy (see
	// ruleCopyByID) — not a replacement for the finding's own specific
	// Message (which stays visible, just de-emphasized under a <details>
	// disclosure): Title/Why answer "what kind of problem is this and why
	// does it matter", Message answers "which exact object, which exact
	// values".
	Title string
	Why   string
	// NextStep is a short, single-sentence restatement of Remediation (its
	// first line) for the card's action rail — the rail is a fast-scan
	// summary, the full Remediation text stays in the card body for anyone
	// who wants the detail.
	NextStep string
	// InspectCommand is the finding's SafeFix.Command, if any — always a
	// read-only kubectl/aws describe-style call (see groupedPlanStep),
	// never an executable fix. Empty when the finding has no SafeFix
	// command, in which case the rail omits the inspect block entirely.
	InspectCommand string
	// InspectElementID is this card's own copy-target <pre> id, scoped by
	// Rank so each card's copy button is unambiguous.
	InspectElementID string
	// FindingTargetID/EvidenceTargetID are the ids of this finding's own
	// row in the Findings tab and Evidence tab (see toHTMLFindings and the
	// Evidence Appendix template), used by the card's "View full finding"/
	// "View evidence" buttons to jump there. Built from Fingerprint, the
	// one identifier guaranteed unique and stable per finding.
	FindingTargetID  string
	EvidenceTargetID string
}

// htmlStartHereItem is one line of the "Start here" fix-order box — the
// same ordering Next Actions already uses (blockers before warnings,
// worst first), just distilled to "what" + "where" with no remediation
// prose, so an operator gets the shape of the work in one glance before
// deciding whether to read further.
type htmlStartHereItem struct {
	Title         string
	ResourceLabel string
}

type htmlUpgradeDetailHop struct {
	From         string
	To           string
	StatusLabel  string
	StatusClass  string
	Assessment   string
	Checks       []string
	FindingLines []string
}

type htmlViewData struct {
	Cluster       string
	Current       string
	Target        string
	UpgradePath   string
	UpgradeLabel  string
	UpgradeLine   string
	CurrentNote   string
	Provider      string
	ProviderLabel string
	AWSEnrichment bool
	// AWSEnrichmentLabel is "On"/"Off" — the banner-meta chip previously
	// rendered the raw Go bool ("true"/"false"), which reads as an
	// internal/debug value rather than operator-facing metadata.
	AWSEnrichmentLabel string
	NamespaceAllowlist string
	ScannedAt          string
	Result             string
	ResultClass        string
	Decision           string
	WhyLine            string
	// HeroTitle/HeroSubtext/HeroExplain are the primary, plain-English
	// framing above the fold (see heroCopy) — Decision/Result/WhyLine
	// remain the authoritative technical labels (GO/REVIEW/NO-GO,
	// CLEAN/BLOCKED/etc.) and are still rendered, just as a secondary
	// badge rather than the first thing a reader sees.
	HeroTitle           string
	HeroSubtext         string
	HeroExplain         string
	Blockers            int
	Warnings            int
	Infos               int
	TotalFindings       int
	GlobalBlockerCount  int
	CoverageIssues      []htmlCoverageIssue
	Assumptions         []string
	ConfidenceMix       []htmlConfidenceStat
	UpgradeDetails      []htmlUpgradeDetailHop
	UpgradeChecks       []string
	StartHere           []htmlStartHereItem
	TopRisks            []htmlTopRisk
	BlockerFindings     []htmlFinding
	WarningFindings     []htmlFinding
	InfoFindings        []htmlFinding
	NextActions         []htmlNextAction
	NextActionsPreview  []htmlNextAction
	NextActionsOverflow int
	AllFindings         []htmlFinding
	// Plan is nil for every scan-produced report (WriteHTML never sets
	// it) — the template's {{if .Plan}} Upgrade Path section stays
	// hidden. Only WritePlanHTML (plan_html.go) populates it.
	Plan *htmlPlanOverview
	// EKSCluster is nil for every non-EKS scan and for an EKS scan where
	// AWS enrichment was unavailable — see findings.EKSClusterInfo.
	EKSCluster *htmlEKSCluster
	// EKSAddons is nil under the same conditions as EKSCluster, or when
	// the cluster has zero installed EKS-managed add-ons.
	EKSAddons []htmlEKSAddon
	// ShowEKSNodegroups is true for EKS scans where managed node group
	// inventory was available, even if ListNodegroups returned zero names.
	ShowEKSNodegroups bool
	EKSNodegroups     []htmlEKSNodegroup
	// ShowEKSUpgradeInsights is true for EKS scans where insight inventory
	// was available, empty, or explicitly unavailable.
	ShowEKSUpgradeInsights        bool
	EKSUpgradeInsights            []htmlEKSUpgradeInsight
	EKSUpgradeInsightsUnavailable bool
	APICompatibility              *findings.APICompatibilitySummary
	UpgradeReadiness              *findings.UpgradeReadinessSummary
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
	return executeHTML(w, data)
}

func executeHTML(w io.Writer, data htmlViewData) error {
	var rendered bytes.Buffer
	if err := htmlTmpl.Execute(&rendered, data); err != nil {
		return err
	}
	lines := strings.Split(rendered.String(), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	_, err := io.WriteString(w, strings.Join(lines, "\n"))
	return err
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
	infos := filterAndSort(r.Findings, findings.SeverityInfo)

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

	// StartHere mirrors the preview items — same order, same items — just
	// distilled to title+resource with no remediation prose, as a
	// glanceable "here's the shape of the work" box above Top Risks. Only
	// meaningful when there's something to fix; a clean/no-warnings
	// report has no Next Actions at all, so this is naturally empty then.
	startHere := make([]htmlStartHereItem, len(preview))
	for i, a := range preview {
		startHere[i] = htmlStartHereItem{Title: a.Title, ResourceLabel: a.ResourceLabel}
	}

	globalBlockerCount := 0
	for _, f := range r.Findings {
		if f.GlobalBlocker {
			globalBlockerCount++
		}
	}
	hasGlobalBlocker := globalBlockerCount > 0

	heroTitle, heroSubtext, heroExplain := heroCopy(r.Result(), r.Summary.Blockers, r.Summary.Warnings, r.TargetVersion)
	awsEnrichmentOn := awsEnrichment(r)
	currentVersion, upgradePath, upgradeLabel, upgradeLine, currentNote := upgradeContextCopy(r.CurrentVersion, r.TargetVersion)

	return htmlViewData{
		Cluster:                       orDash(r.ClusterContext),
		Current:                       currentVersion,
		Target:                        r.TargetVersion,
		UpgradePath:                   upgradePath,
		UpgradeLabel:                  upgradeLabel,
		UpgradeLine:                   upgradeLine,
		CurrentNote:                   currentNote,
		Provider:                      providerLabel,
		ProviderLabel:                 providerDisplayLabel(providerLabel),
		AWSEnrichment:                 awsEnrichmentOn,
		AWSEnrichmentLabel:            awsEnrichmentLabel(awsEnrichmentOn),
		NamespaceAllowlist:            strings.Join(r.NamespaceAllowlist, ", "),
		ScannedAt:                     r.ScannedAt.Format("2006-01-02 15:04:05 MST"),
		Result:                        r.Result(),
		ResultClass:                   resultClass(r.Result()),
		Decision:                      decisionLabel(r.Result()),
		WhyLine:                       reportDecisionWhyLine(r),
		HeroTitle:                     heroTitle,
		HeroSubtext:                   heroSubtext,
		HeroExplain:                   heroExplain,
		Blockers:                      r.Summary.Blockers,
		Warnings:                      r.Summary.Warnings,
		Infos:                         r.Summary.Infos,
		TotalFindings:                 len(r.Findings),
		GlobalBlockerCount:            globalBlockerCount,
		CoverageIssues:                htmlCoverageIssues(r),
		Assumptions:                   r.Assumptions,
		ConfidenceMix:                 confidenceMix(r.Findings),
		UpgradeDetails:                htmlUpgradeDetails(r),
		UpgradeChecks:                 upgradeCheckLines(),
		StartHere:                     startHere,
		TopRisks:                      toHTMLTopRisks(topRisks(r.Findings, 3)),
		BlockerFindings:               toHTMLFindings(blockers, "blocker", hasGlobalBlocker),
		WarningFindings:               toHTMLFindings(warnings, "warning", hasGlobalBlocker),
		InfoFindings:                  toHTMLFindings(infos, "info", hasGlobalBlocker),
		NextActions:                   nextActions,
		NextActionsPreview:            preview,
		NextActionsOverflow:           overflow,
		AllFindings:                   toHTMLFindings(allSorted(r.Findings), "all", hasGlobalBlocker),
		EKSCluster:                    toHTMLEKSCluster(r.EKSCluster),
		EKSAddons:                     toHTMLEKSAddons(r.EKSAddons),
		ShowEKSNodegroups:             showEKSNodegroups(r),
		EKSNodegroups:                 toHTMLEKSNodegroups(r.EKSNodegroups),
		ShowEKSUpgradeInsights:        showEKSUpgradeInsights(r),
		EKSUpgradeInsights:            toHTMLEKSUpgradeInsights(r.EKSUpgradeInsights),
		EKSUpgradeInsightsUnavailable: eksUpgradeInsightsUnavailable(r),
		APICompatibility:              r.APICompatibility,
		UpgradeReadiness:              r.UpgradeReadiness,
	}
}

func htmlUpgradeDetails(r *findings.Report) []htmlUpgradeDetailHop {
	path, _, ok := findings.UpgradePath(r.CurrentVersion, r.TargetVersion)
	if !ok || len(path) < 2 {
		return nil
	}
	multiHop := len(path) > 2
	out := make([]htmlUpgradeDetailHop, 0, len(path)-1)
	for i := 0; i < len(path)-1; i++ {
		hop := htmlUpgradeDetailHop{
			From:   path[i],
			To:     path[i+1],
			Checks: upgradeCheckLines(),
		}
		if i == 0 && !multiHop {
			hop.StatusLabel, hop.StatusClass, hop.Assessment = currentHopStatus(r)
			hop.FindingLines = currentHopFindingLines(r.Findings)
		} else if i == 0 {
			hop.StatusLabel = "Planned, hop-specific scan recommended"
			hop.StatusClass = "rescan-required"
			hop.Assessment = fmt.Sprintf("Findings were evaluated against final target %s, not this individual hop. Re-scan or run plan for a hop-specific assessment.", r.TargetVersion)
			hop.FindingLines = []string{"Overall target blockers remain listed in this report, but they are not proof that this intermediate hop is blocked."}
		} else {
			hop.StatusLabel = "Planned, re-scan required"
			hop.StatusClass = "rescan-required"
			hop.Assessment = "Do not treat this future hop as safe yet. Complete the previous hop, then re-run KubePreflight against this target."
			hop.FindingLines = []string{fmt.Sprintf("Findings were evaluated against final target %s; current findings are not projected as proof for this future cluster state.", r.TargetVersion)}
		}
		out = append(out, hop)
	}
	return out
}

func currentHopStatus(r *findings.Report) (label, class, assessment string) {
	switch {
	case r.Summary.Blockers > 0:
		return "Blocked", "blocked", "Current findings must be resolved before this hop should proceed."
	case r.Summary.Warnings > 0:
		return "Needs review", "warning", "No hard blockers were found, but warnings should be reviewed before this hop."
	default:
		return "Current assessment", "current-live", "No blockers or warnings were found for the currently assessed hop."
	}
}

func upgradeCheckLines() []string {
	return []string{
		"API removals and deprecated API usage",
		"Node/kubelet version skew",
		"Admission webhook availability and scope",
		"PDB and workload drain safety",
		"Add-on, CoreDNS, CNI, and storage/CSI compatibility",
		"Release notes review for the target minor",
	}
}

func currentHopFindingLines(fs []findings.Finding) []string {
	counts := map[string]map[findings.Severity]int{}
	ruleIDs := map[string]map[string]bool{}
	for _, f := range fs {
		category := upgradeCategoryForRule(f.RuleID)
		if counts[category] == nil {
			counts[category] = map[findings.Severity]int{}
			ruleIDs[category] = map[string]bool{}
		}
		counts[category][f.Severity]++
		ruleIDs[category][f.RuleID] = true
	}
	if len(counts) == 0 {
		return []string{"No current findings mapped to hop risk categories."}
	}
	categories := make([]string, 0, len(counts))
	for category := range counts {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	lines := make([]string, 0, len(categories))
	for _, category := range categories {
		severityCounts := counts[category]
		parts := []string{}
		if severityCounts[findings.SeverityBlocker] > 0 {
			parts = append(parts, fmt.Sprintf("%d blocker(s)", severityCounts[findings.SeverityBlocker]))
		}
		if severityCounts[findings.SeverityWarning] > 0 {
			parts = append(parts, fmt.Sprintf("%d warning(s)", severityCounts[findings.SeverityWarning]))
		}
		if severityCounts[findings.SeverityInfo] > 0 {
			parts = append(parts, fmt.Sprintf("%d info", severityCounts[findings.SeverityInfo]))
		}
		ids := make([]string, 0, len(ruleIDs[category]))
		for id := range ruleIDs[category] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		lines = append(lines, fmt.Sprintf("%s: %s (%s)", category, strings.Join(parts, ", "), strings.Join(ids, ", ")))
	}
	return lines
}

func upgradeCategoryForRule(ruleID string) string {
	switch ruleID {
	case "API-001", "API-002", "CRD-001", "EKS-INSIGHT-001", "EKS-INSIGHT-002", "EKS-INSIGHT-003":
		return "API removals and deprecations"
	case "NODE-001":
		return "Node/kubelet skew"
	case "NODE-003":
		return "Node scheduling compatibility"
	case "WH-001", "WH-002":
		return "Admission webhooks"
	case "PDB-001", "PDB-002":
		return "PDB and drain safety"
	case "ADDON-001", "COREDNS-001", "NODE-002", "EKS-NG-001", "EKS-NG-002", "EKS-NG-003", "EKS-NG-004":
		return "Add-on and platform compatibility"
	default:
		return "Other upgrade readiness checks"
	}
}

func upgradeContextCopy(currentVersion, targetVersion string) (current, path, label, line, note string) {
	current = "Unknown"
	if normalized, ok := findings.NormalizeKubernetesVersion(currentVersion); ok {
		current = normalized
	}
	pathParts, label, ok := findings.UpgradePath(current, targetVersion)
	if ok {
		path = strings.Join(pathParts, " \u2192 ")
		line = fmt.Sprintf("This scan checks readiness for upgrading from %s to %s.", current, targetVersion)
		return current, path, label, line, ""
	}
	path = current + " \u2192 " + targetVersion
	if current == "Unknown" {
		note = "Current control-plane version was not available from the Kubernetes server version API. Node/kubelet versions are evaluated separately."
		line = fmt.Sprintf("This scan checks readiness for target %s; current control-plane version is unknown.", targetVersion)
		return current, path, label, line, note
	}
	line = fmt.Sprintf("This scan checks readiness for target %s; upgrade path could not be derived from current version %s.", targetVersion, current)
	return current, path, label, line, ""
}

func resultClass(result string) string {
	switch result {
	case "BLOCKED":
		return "blocked"
	case "PASSED_WITH_WARNINGS":
		return "warn"
	case "INCOMPLETE":
		return "warn"
	default:
		return "clean"
	}
}

// heroCopy is the plain-English framing shown first, above the technical
// GO/REVIEW/NO-GO badge — a first-time reader shouldn't have to already
// know what "NO-GO" or "BLOCKED" mean to understand that an upgrade needs
// work before it's safe to start. Purely presentational: Result/Decision/
// WhyLine (the authoritative labels) are unchanged and still rendered,
// just as secondary content below this.
func heroCopy(result string, blockers, warnings int, target string) (title, subtext, explain string) {
	switch result {
	case "BLOCKED":
		title = "Upgrade blocked"
		subtext = fmt.Sprintf("%d %s required before upgrading to %s", blockers, pluralize(blockers, "fix", "fixes"), target)
		explain = "KubePreflight found issues that can cause node drain or upgrade failure. Fix these before starting the change window."
	case "PASSED_WITH_WARNINGS":
		title = "Upgrade needs review"
		subtext = fmt.Sprintf("%d %s to review before upgrading to %s", warnings, pluralize(warnings, "item", "items"), target)
		explain = "KubePreflight found lower-risk issues. Review them, then proceed with the change window."
	case "INCOMPLETE":
		title = "Assessment incomplete"
		subtext = fmt.Sprintf("Evidence could not be fully collected for upgrading to %s", target)
		explain = "KubePreflight could not verify every check. Resolve the coverage errors below and rerun before upgrading."
	default:
		title = "Ready to upgrade"
		subtext = fmt.Sprintf("No blockers found for upgrading to %s", target)
		explain = "KubePreflight found no blockers or warnings for this upgrade."
	}
	return title, subtext, explain
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// severityActionLabel is the fuller, unambiguous severity wording used in
// the Summary tab's Top Risks cards and Start Here box — "BLOCKER" alone
// doesn't tell a first-time reader whether it's safe to proceed; the
// dense Findings-tab list keeps the shorter plain severity-pill text
// unchanged, since that view's whole layout is already information-dense
// by design.
func severityActionLabel(sev findings.Severity) string {
	switch sev {
	case findings.SeverityBlocker:
		return "BLOCKER — must fix before upgrade"
	case findings.SeverityWarning:
		return "WARNING — review before upgrade"
	default:
		return "INFO — no action required"
	}
}

// ruleCopy is short, rule-general operator copy — not a replacement for a
// finding's own specific Message/Remediation (both stay, generated fresh
// per finding with real resource names/values), just a plain-English
// label for "what kind of problem is this" and "why does it block an
// upgrade" that doesn't require already knowing what e.g. "PDB-001" means.
type ruleCopy struct{ Title, Why string }

var ruleCopyByID = map[string]ruleCopy{
	"API-001": {
		Title: "Removed API version",
		Why:   "This resource uses a Kubernetes API version that will be removed at your target version. Once removed, applying or updating this resource fails.",
	},
	"API-002": {
		Title: "Deprecated API version",
		Why:   "This resource uses a Kubernetes API version that is still served at your target version, but has a known future removal. Migrate it before the next incompatible upgrade.",
	},
	"EKS-INSIGHT-001": {
		Title: "EKS Upgrade Insight reports ERROR",
		Why:   "Amazon EKS reported an AWS-native upgrade readiness concern. This PR treats ERROR as a warning until blocker policy has real-cluster validation.",
	},
	"EKS-INSIGHT-002": {
		Title: "EKS Upgrade Insight reports WARNING",
		Why:   "Amazon EKS reported an AWS-native upgrade readiness warning for this target Kubernetes version.",
	},
	"EKS-INSIGHT-003": {
		Title: "EKS Upgrade Insight status unknown",
		Why:   "Amazon EKS could not provide a clear passing or failing status for this upgrade insight.",
	},
	"WH-001": {
		Title: "Overly broad webhook scope",
		Why:   "This admission webhook matches almost any resource with no narrowing selector, putting it on the critical path for far more API writes than it needs to be — including ones made during the upgrade.",
	},
	"WH-002": {
		Title: "Webhook backend is down",
		Why:   "This fail-closed admission webhook has no healthy backend. While it's down, Kubernetes rejects every API write the webhook is supposed to validate — including writes needed to complete the upgrade.",
	},
	"WORKLOAD-001": {
		Title: "Workload already unhealthy",
		Why:   "This pod or workload was already unhealthy before the upgrade. Fix it or document an explicit waiver so post-upgrade validation is not confused by pre-existing application breakage.",
	},
	"PDB-001": {
		Title: "Pod cannot be safely evicted",
		Why:   "This PodDisruptionBudget currently allows zero voluntary evictions. During a node drain, Kubernetes cannot evict the matching pods, so the upgrade can stall or fail.",
	},
	"PDB-002": {
		Title: "Conflicting PodDisruptionBudgets",
		Why:   "Two PodDisruptionBudgets match the same pod. The Eviction API rejects an eviction when more than one budget matches it, even if each one individually would allow it.",
	},
	"NODE-001": {
		Title: "Node version is too old",
		Why:   "This node's kubelet version is outside the supported version-skew window for your target Kubernetes version. Nodes outside that window can fail to operate correctly once the control plane is upgraded.",
	},
	"NODE-002": {
		Title: "Not enough IP capacity for the upgrade",
		Why:   "An EKS control-plane upgrade creates additional network interfaces in this subnet, and there isn't enough free IP headroom left for them.",
	},
	"NODE-003": {
		Title: "Deprecated master node label",
		Why:   "This workload still schedules against node-role.kubernetes.io/master. New or rebuilt control-plane nodes may carry only node-role.kubernetes.io/control-plane, so the workload can fail to schedule after an upgrade or node replacement.",
	},
	"NET-002": {
		Title: "Referenced network resource is missing",
		Why:   "This cluster references a security group or VPC that no longer exists. AWS documents this as a hard EKS control-plane upgrade failure, not a soft warning.",
	},
	"ADDON-001": {
		Title: "Add-on not compatible with target version",
		Why:   "AWS hasn't listed this add-on's currently-installed version as compatible with your target Kubernetes version.",
	},
	"EKS-NG-001": {
		Title: "Managed node group has health issues",
		Why:   "Amazon EKS reports managed node group health issues that can complicate node replacement or upgrade operations.",
	},
	"EKS-NG-002": {
		Title: "Managed node group has limited update headroom",
		Why:   "This managed node group is already at its minimum size, so rolling replacement may have little disruption headroom.",
	},
	"EKS-NG-003": {
		Title: "Managed node group needs manual AMI review",
		Why:   "Launch-template or custom-AMI node groups need manual validation of AMI, bootstrap, kubelet, and launch template upgrade behavior.",
	},
	"EKS-NG-004": {
		Title: "Managed node group version context",
		Why:   "AWS reports this managed node group on an older Kubernetes version than the target. Actual kubelet skew is evaluated separately by NODE-001.",
	},
	"COREDNS-001": {
		Title: "CoreDNS health check is incomplete",
		Why:   "This CoreDNS configuration is missing the `ready` plugin, so its readiness probe can't reflect actual DNS health — a known trap that tends to surface only after an upgrade.",
	},
	"CRD-001": {
		Title: "Custom resources need storage-version migration",
		Why:   "This CRD still has objects stored outside the current storage version. If that stored version is no longer served by the CRD, reads and conversion paths can fail until the objects are migrated.",
	},
	"CRD-002": {
		Title: "CRD conversion webhook is down",
		Why:   "This CRD's conversion webhook has no healthy backend. Reading or updating existing objects in this CRD can require conversion during the upgrade, so this is a hard blocker, not a soft warning.",
	},
	"APISERVICE-001": {
		Title: "Extension API is unavailable",
		Why:   "This aggregated APIService isn't reporting Available. Aggregated API discovery or requests can fail during upgrade validation and controller reconciliation.",
	},
}

func ruleTitle(ruleID string) string {
	if c, ok := ruleCopyByID[ruleID]; ok {
		return c.Title
	}
	return ruleID
}

func ruleWhy(ruleID string) string {
	if c, ok := ruleCopyByID[ruleID]; ok {
		return c.Why
	}
	return "This finding was flagged as a risk for the target upgrade version."
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
	case "INCOMPLETE":
		return "NO-GO"
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

func reportDecisionWhyLine(r *findings.Report) string {
	if r.Result() == "INCOMPLETE" {
		if r.Summary.Blockers > 0 {
			plural := "s"
			if r.Summary.Blockers == 1 {
				plural = ""
			}
			return fmt.Sprintf("Assessment incomplete — %d blocker%s observed with available evidence. Resolve coverage errors and rerun before upgrading.", r.Summary.Blockers, plural)
		}
		return "Assessment incomplete — evidence collection was incomplete. Resolve coverage errors and rerun before upgrading."
	}
	return decisionWhyLine(r.Summary.Blockers, r.Summary.Warnings)
}

func htmlCoverageIssues(r *findings.Report) []htmlCoverageIssue {
	planes := []struct {
		name     string
		coverage findings.PlaneCoverage
	}{{"Kubernetes", r.Coverage.Kubernetes}, {"AWS", r.Coverage.AWS}, {"Manifests", r.Coverage.Manifests}}
	var out []htmlCoverageIssue
	for _, plane := range planes {
		if plane.coverage.Status == findings.CoveragePartial {
			out = append(out, htmlCoverageIssue{Plane: plane.name, Errors: plane.coverage.Errors})
		}
	}
	return out
}

// topRisks: Priority first, Severity second, Confidence third, rule
// ID/resource last (see findingLess, view.go) — the same order every
// other renderer uses, just capped for an above-the-fold executive
// summary. Not a scoring model.
func topRisks(fs []findings.Finding, limit int) []findings.Finding {
	sorted := make([]findings.Finding, len(fs))
	copy(sorted, fs)
	sort.Slice(sorted, func(i, j int) bool { return findingLess(sorted[i], sorted[j]) })
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
	if r.Coverage.AWS.Status == findings.CoverageComplete {
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

func awsEnrichmentLabel(on bool) string {
	if on {
		return "On"
	}
	return "Off"
}

// providerDisplayLabel gives known providers their conventional casing
// (an all-lowercase "eks"/"cluster-only" reads as an internal enum value,
// not something written for an operator to read) — anything unrecognized
// falls back to just capitalizing the first letter rather than guessing.
func providerDisplayLabel(provider string) string {
	switch provider {
	case "eks":
		return "EKS"
	case "aks":
		return "AKS"
	case "gke":
		return "GKE"
	case "cluster-only", "":
		return "Cluster-only"
	default:
		return strings.ToUpper(provider[:1]) + provider[1:]
	}
}

// htmlEKSCluster is report.html's view of findings.EKSClusterInfo — nil
// whenever the underlying field is nil (non-EKS scan, or EKS enrichment
// unavailable), in which case the template's {{if .EKSCluster}} banner
// chips stay hidden entirely rather than showing empty values.
type htmlEKSCluster struct {
	ClusterName         string
	Region              string
	Version             string
	PlatformVersion     string
	Status              string
	SupportType         string
	SupportTypeLabel    string
	EndpointAccess      string
	EndpointAccessLabel string
}

func toHTMLEKSCluster(c *findings.EKSClusterInfo) *htmlEKSCluster {
	if c == nil {
		return nil
	}
	return &htmlEKSCluster{
		ClusterName:         c.ClusterName,
		Region:              c.Region,
		Version:             c.Version,
		PlatformVersion:     c.PlatformVersion,
		Status:              c.Status,
		SupportType:         c.SupportType,
		SupportTypeLabel:    eksSupportTypeLabel(c.SupportType),
		EndpointAccess:      c.EndpointAccess,
		EndpointAccessLabel: eksEndpointAccessLabel(c.EndpointAccess),
	}
}

func eksSupportTypeLabel(supportType string) string {
	switch supportType {
	case "EXTENDED":
		return "Extended support"
	case "STANDARD":
		return "Standard support"
	default:
		return ""
	}
}

func eksEndpointAccessLabel(access string) string {
	switch access {
	case "public":
		return "Public"
	case "private":
		return "Private"
	case "public_and_private":
		return "Public + private"
	default:
		return ""
	}
}

// htmlEKSAddon is report.html's view of one findings.EKSAddonInfo entry —
// the full add-on inventory ADDON-001 only partially surfaces (it raises a
// finding for incompatible add-ons, but a compatible one produces no
// finding at all and is otherwise invisible in the report).
type htmlEKSAddon struct {
	Name               string
	CurrentVersion     string
	CompatibleVersions string
	StatusLabel        string
	StatusClass        string // "clean", "warn", or "blocked" — matches the existing .badge-* classes
}

func toHTMLEKSAddons(addons []findings.EKSAddonInfo) []htmlEKSAddon {
	if len(addons) == 0 {
		return nil
	}
	out := make([]htmlEKSAddon, len(addons))
	for i, a := range addons {
		label, class := eksAddonStatus(a)
		out[i] = htmlEKSAddon{
			Name:               a.Name,
			CurrentVersion:     a.CurrentVersion,
			CompatibleVersions: strings.Join(a.CompatibleVersions, ", "),
			StatusLabel:        label,
			StatusClass:        class,
		}
	}
	return out
}

func eksAddonStatus(a findings.EKSAddonInfo) (label, class string) {
	switch {
	case a.VerificationUnavailable:
		return "Verification unavailable", "warn"
	case a.Compatible:
		return "Compatible", "clean"
	default:
		return "Needs update", "blocked"
	}
}

type htmlEKSNodegroup struct {
	Name           string
	Status         string
	Version        string
	Release        string
	AMIType        string
	CapacityType   string
	Scaling        string
	UpdateConfig   string
	Health         string
	Readiness      string
	ReadinessClass string
}

func showEKSNodegroups(r *findings.Report) bool {
	if len(r.EKSNodegroups) > 0 {
		return true
	}
	if r.Provider != "eks" || r.EKSCluster == nil {
		return false
	}
	for _, err := range r.Coverage.AWS.Errors {
		if strings.Contains(err, "list-nodegroups") || strings.Contains(err, "describe-nodegroup:") {
			return false
		}
	}
	return true
}

func toHTMLEKSNodegroups(nodegroups []findings.EKSNodegroupInfo) []htmlEKSNodegroup {
	if len(nodegroups) == 0 {
		return nil
	}
	out := make([]htmlEKSNodegroup, 0, len(nodegroups))
	for _, ng := range nodegroups {
		out = append(out, htmlEKSNodegroup{
			Name:           ng.Name,
			Status:         emptyDash(ng.Status),
			Version:        emptyDash(ng.Version),
			Release:        emptyDash(ng.ReleaseVersion),
			AMIType:        emptyDash(ng.AMIType),
			CapacityType:   emptyDash(ng.CapacityType),
			Scaling:        scalingConfigLabel(ng.DesiredSize, ng.MinSize, ng.MaxSize),
			UpdateConfig:   updateConfigLabel(ng.MaxUnavailable, ng.MaxUnavailablePercentage),
			Health:         nodegroupHealthLabel(ng.HealthIssues),
			Readiness:      emptyDash(ng.ReadinessStatus),
			ReadinessClass: nodegroupReadinessClass(ng.ReadinessStatus),
		})
	}
	return out
}

func scalingConfigLabel(desired, min, max *int32) string {
	return fmt.Sprintf("%s / %s / %s", int32Label(desired), int32Label(min), int32Label(max))
}

func updateConfigLabel(maxUnavailable, maxUnavailablePercentage *int32) string {
	switch {
	case maxUnavailable != nil:
		return fmt.Sprintf("maxUnavailable: %d", *maxUnavailable)
	case maxUnavailablePercentage != nil:
		return fmt.Sprintf("maxUnavailable: %d%%", *maxUnavailablePercentage)
	default:
		return "—"
	}
}

func int32Label(v *int32) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%d", *v)
}

func nodegroupHealthLabel(issues []findings.EKSNodegroupHealthIssue) string {
	if len(issues) == 0 {
		return "Healthy"
	}
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		if issue.Code != "" {
			codes = append(codes, issue.Code)
		}
	}
	if len(codes) == 0 {
		return fmt.Sprintf("%d issue(s)", len(issues))
	}
	return strings.Join(codes, ", ")
}

func nodegroupReadinessClass(status string) string {
	if strings.Contains(strings.ToLower(status), "required") {
		return "warn"
	}
	return "clean"
}

type htmlEKSUpgradeInsight struct {
	Name              string
	Status            string
	StatusClass       string
	KubernetesVersion string
	LastRefreshed     string
	Recommendation    string
	Details           string
}

func showEKSUpgradeInsights(r *findings.Report) bool {
	if len(r.EKSUpgradeInsights) > 0 || eksUpgradeInsightsUnavailable(r) {
		return true
	}
	return r.Provider == "eks" && r.EKSCluster != nil
}

func eksUpgradeInsightsUnavailable(r *findings.Report) bool {
	for _, err := range r.Coverage.AWS.Errors {
		if strings.Contains(err, "list-insights") {
			return true
		}
		if len(r.EKSUpgradeInsights) == 0 && strings.Contains(err, "describe-insight:") {
			return true
		}
	}
	return false
}

func toHTMLEKSUpgradeInsights(insights []findings.EKSUpgradeInsightInfo) []htmlEKSUpgradeInsight {
	if len(insights) == 0 {
		return nil
	}
	out := make([]htmlEKSUpgradeInsight, 0, len(insights))
	for _, ins := range insights {
		out = append(out, htmlEKSUpgradeInsight{
			Name:              emptyDash(ins.Name),
			Status:            emptyDash(ins.Status),
			StatusClass:       eksUpgradeInsightStatusClass(ins.Status),
			KubernetesVersion: emptyDash(ins.KubernetesVersion),
			LastRefreshed:     insightTimeLabel(ins.LastRefreshTime, ins.LastTransitionTime),
			Recommendation:    emptyDash(ins.Recommendation),
			Details:           insightDetailsLabel(ins),
		})
	}
	return out
}

func eksUpgradeInsightStatusClass(status string) string {
	switch strings.ToUpper(status) {
	case "ERROR", "WARNING":
		return "warn"
	case "UNKNOWN":
		return "info"
	default:
		return "clean"
	}
}

func insightTimeLabel(refresh, transition string) string {
	switch {
	case refresh != "" && transition != "":
		return refresh + " / " + transition
	case refresh != "":
		return refresh
	case transition != "":
		return "transition: " + transition
	default:
		return "—"
	}
}

func insightDetailsLabel(ins findings.EKSUpgradeInsightInfo) string {
	parts := append([]string{}, ins.DeprecationDetails...)
	parts = append(parts, ins.AddonCompatibility...)
	for k, v := range ins.AdditionalInfo {
		if v != "" {
			parts = append(parts, k+": "+v)
		}
	}
	if len(parts) == 0 {
		return emptyDash(ins.Description)
	}
	return strings.Join(parts, " | ")
}

func emptyDash(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

func confidenceMix(fs []findings.Finding) []htmlConfidenceStat {
	counts := map[findings.ConfidenceTier]int{}
	for _, f := range fs {
		counts[f.Confidence]++
	}
	order := []findings.ConfidenceTier{findings.TierStaticCertain, findings.TierObserved, findings.TierProviderReported, findings.TierInferred}
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
		var inspectCommand string
		if f.RemediationDetail != nil && f.RemediationDetail.SafeFix != nil {
			inspectCommand = f.RemediationDetail.SafeFix.Command
		}
		out[i] = htmlTopRisk{
			htmlFinding:      htmlFinding{Finding: f, ResourceLabel: findingResourceLabel(f), PlaneLabel: planeLabel(f)},
			Rank:             i + 1,
			Title:            ruleTitle(f.RuleID),
			Why:              ruleWhy(f.RuleID),
			NextStep:         firstSentence(f.Remediation),
			InspectCommand:   inspectCommand,
			InspectElementID: fmt.Sprintf("top-risk-%d-inspect", i+1),
			FindingTargetID:  "finding-" + f.Fingerprint,
			EvidenceTargetID: "evidence-" + f.Fingerprint,
		}
	}
	return out
}

// firstSentence extracts a short, single-sentence restatement of a
// finding's remediation text for the Top Risk card's action rail — a
// fast-scan summary, not a replacement for the full Remediation text the
// card body already shows. Unlike firstLine (which only stops at a literal
// newline and can return an entire multi-clause paragraph verbatim when
// there isn't one), this stops at the first sentence boundary so the rail
// doesn't just duplicate the card body word-for-word. Falls back to a
// length-capped, word-boundary-safe prefix when no sentence boundary
// appears within a reasonable scan window.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	const maxScan = 180
	scan := s
	truncated := false
	if len(scan) > maxScan {
		scan = scan[:maxScan]
		truncated = true
	}
	for i := 0; i < len(scan); i++ {
		if scan[i] == '.' && (i+1 == len(scan) || scan[i+1] == ' ') {
			return scan[:i+1]
		}
	}
	if !truncated {
		return scan
	}
	if i := strings.LastIndexByte(scan, ' '); i > 0 {
		scan = scan[:i]
	}
	return scan + "…"
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

		var command string
		if a.Primary.RemediationDetail != nil && a.Primary.RemediationDetail.SafeFix != nil {
			command = a.Primary.RemediationDetail.SafeFix.Command
		}

		out[i] = htmlNextAction{
			ResourceLabel:  a.ResourceLabel,
			RuleIDsJoined:  strings.Join(a.RuleIDs, ", "),
			Title:          ruleTitle(a.Primary.RuleID),
			Severity:       a.Severity,
			Remediation:    a.Primary.Remediation,
			Related:        related,
			ElementID:      fmt.Sprintf("action-%d", i),
			GroupedPlan:    groupedPlan,
			Command:        command,
			Priority:       a.Primary.Priority,
			PriorityReason: a.Primary.PriorityReason,
		}
	}
	return out
}

// groupedPlanStep renders one finding as a single actionable step for a
// merged Next Action's grouped plan: the structured safe-fix command when
// available, falling back to the plain remediation text's first line for
// findings without a RemediationDetail.
//
// Every rule's SafeFix.Command today is a read-only kubectl get/describe
// (or aws ... describe-*) call, not an executable fix — it exists to help
// a human confirm current state before deciding what to change, same as
// the Findings tab's "Inspect first" labeling and the Summary preview's
// command block. Labeling it plainly here too keeps a copy-pasted grouped
// plan from reading like "run these three commands and you're done" when
// step 1 is actually just gathering evidence.
func groupedPlanStep(f findings.Finding) string {
	if f.RemediationDetail != nil && f.RemediationDetail.SafeFix != nil {
		if f.RemediationDetail.SafeFix.Command != "" {
			return fmt.Sprintf("[%s] Inspect: %s", f.RuleID, f.RemediationDetail.SafeFix.Command)
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
  .hero-title { margin: 0; color: white; font-size: clamp(22px, 3.6vw, 30px); font-weight: 700; letter-spacing: -.01em; }
  .hero-subtext { margin: 6px 0 0; color: white; font-size: 15px; font-weight: 600; }
  .hero-badge-row { display: flex; align-items: center; gap: 10px; margin-top: 10px; }
  .hero-report-name { color: #8ca49e; font-size: 11px; text-transform: uppercase; letter-spacing: .08em; }
  .why-line { margin: 10px 0 0; padding-top: 10px; border-top: 1px solid rgba(255,255,255,.14); color: #dfeae6; font-size: 13px; }
  .hero-explain { margin: 6px 0 0; max-width: 640px; color: #c3d6cf; font-size: 13.5px; line-height: 1.55; }
  .upgrade-context-line { margin: 8px 0 0; color: white; font-size: 13.5px; font-weight: 600; }
  .upgrade-context-note { margin: 4px 0 0; max-width: 760px; color: #c3d6cf; font-size: 12.5px; line-height: 1.45; }
  .banner-meta { display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 10px; margin: 16px 0 0; padding-top: 14px; border-top: 1px solid rgba(255,255,255,.14); }
  .meta-chip { padding: 9px 12px; border: 1px solid rgba(255,255,255,.14); border-radius: var(--radius-sm); background: rgba(255,255,255,.04); }
  .meta-chip-wide { grid-column: span 2; }
  .banner-meta dt { color: #8ca49e; font-size: 10px; text-transform: uppercase; letter-spacing: .1em; }
  .banner-meta dd { margin: 4px 0 0; font: 13px monospace; }
  .meta-subtle { display: block; margin-top: 3px; color: #8ca49e; font: 11px Inter, ui-sans-serif, system-ui, sans-serif; text-transform: none; letter-spacing: 0; }

  .badge { display: inline-block; padding: 6px 9px; border: 1px solid currentColor; border-radius: var(--radius-sm); font-size: 10.5px; font-weight: 700; letter-spacing: .08em; }
  .badge.blocked { color: #ffaaa1; } .badge.warn { color: #ffd28c; } .badge.clean { color: var(--mint); }

  .summary-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 12px; }
  .metric { display: block; width: 100%; padding: 14px 16px; border: 1px solid var(--line); border-top: 3px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); color: inherit; font: inherit; text-align: left; }
  .metric-button { cursor: pointer; transition: border-color .1s, box-shadow .1s, transform .1s; }
  .metric-button:hover { border-color: #b9b4a6; box-shadow: 0 10px 24px rgba(29,36,32,.16); transform: translateY(-1px); }
  .metric-button:focus-visible { outline: 2px solid var(--navy); outline-offset: 2px; }
  .metric[aria-disabled="true"] { color: var(--muted); box-shadow: none; }
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
  .upgrade-details-list { list-style: none; margin: 10px 0 0; padding: 0; display: grid; gap: 8px; }
  .upgrade-detail-card { padding: 10px 12px; border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); }
  .upgrade-detail-card.blocked { border-left-color: var(--red); }
  .upgrade-detail-card.warning { border-left-color: var(--amber); }
  .upgrade-detail-card.current-live { border-left-color: var(--mint); }
  .upgrade-detail-card.rescan-required { border-left-color: var(--blue); }
  .upgrade-detail-head { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  .upgrade-detail-body { margin-top: 8px; }
  .upgrade-detail-body h3 { margin: 0 0 4px; font-size: 11px; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  .upgrade-detail-body p { margin: 0; color: var(--muted); font-size: 13px; line-height: 1.5; }
  .upgrade-detail-body ul { margin: 0; padding-left: 18px; color: var(--muted); font-size: 13px; line-height: 1.5; }
  .upgrade-checks-details { margin-top: 10px; border-top: 1px solid var(--line); padding-top: 10px; color: var(--muted); font-size: 13px; }
  .upgrade-checks-details summary { cursor: pointer; color: var(--ink); font-weight: 700; }
  .upgrade-checks-details ul { margin: 8px 0 0; padding-left: 18px; line-height: 1.5; }
  .badge-current-live, .badge-projected, .badge-rescan-required { display: inline-flex; align-items: center; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; }
  .badge-current-live { background: var(--mint); color: #0c3d2c; }
  .badge-projected { background: var(--blue-soft); color: var(--blue); }
  .badge-rescan-required { background: var(--amber-soft); color: #754706; }
  .badge-blocked { background: var(--red-soft); color: #8e2d25; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .badge-warning { background: var(--amber-soft); color: #754706; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .badge-warn { background: var(--amber-soft); color: #754706; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .badge-clean { background: #e3f5ee; color: #146c50; padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .badge-info { background: var(--blue-soft); color: var(--blue); padding: 4px 8px; font-size: 10px; font-weight: 700; }
  .rule-id-chip { display: inline-flex; align-items: center; background: var(--blue-soft); color: var(--blue); border: none; border-radius: 4px; padding: 3px 7px; margin: 1px 2px 1px 0; font-size: 11px; font-weight: 700; font-family: monospace; cursor: pointer; }
  .rule-id-chip:hover { background: var(--blue); color: white; }
  .carry-forward-list { flex: 1 1 100%; margin: 4px 0 0; padding-left: 18px; font-size: 12.5px; color: var(--muted); }
  .upgrade-path-caption { margin: 10px 0 0; font-size: 12.5px; color: var(--muted); }

  .global-blocker-banner { border: 1px solid var(--red); border-left: 4px solid var(--red); border-radius: var(--radius); background: var(--red-soft); padding: 14px 16px; }
  .global-blocker-banner h2 { margin: 0; font: 700 15px/1.3 Inter, ui-sans-serif, system-ui, sans-serif; color: #8e2d25; }
  .global-blocker-banner p { margin: 6px 0 0; font-size: 13.5px; color: #6b241d; }
  .global-blocker-count { font-weight: 700; }
  .global-blocker-badge { display: inline-flex; align-items: center; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; background: var(--red); color: white; }
  .dependency-warning { margin: 6px 0 0; padding: 6px 10px; font-size: 12.5px; color: #6b241d; background: var(--red-soft); border-left: 3px solid var(--red); }
  .priority-detail { margin: 0 0 10px; padding: 8px 12px; border-left: 3px solid var(--line); background: #f7f6f0; font-size: 12.5px; color: var(--ink); }
  .priority-detail.p1 { border-left-color: var(--red); }
  .priority-detail.p2 { border-left-color: var(--amber); }
  .priority-detail.p3 { border-left-color: var(--blue); }
  .priority-detail strong { display: block; font-size: 10.5px; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); }
  .priority-detail p { margin: 4px 0 0; }
  .priority-detail .priority-meta { color: var(--muted); }

  /* Long-form explanatory text (risk cards, next-action prose) reads
     poorly at full container width — capped so line length stays
     comfortable regardless of how wide the surrounding card/viewport is.
     overflow-wrap: anywhere because a finding's Remediation can embed a
     copy-pasteable command whose longest token (e.g. a kubectl patch
     -p='[{...}]' JSON payload) has zero natural break opportunities — on
     a phone-width viewport an unbreakable ~140-char token otherwise
     forces the whole preview list wider than the page and gets clipped
     by the mobile html{overflow-x:hidden} rule. */
  .risk-body { max-width: 1100px; margin: 4px 0 0; line-height: 1.55; color: var(--ink); overflow-wrap: anywhere; }
  .risk-body.risk-reason { color: var(--muted); }

  .start-here { padding: 14px 18px; border: 1px solid var(--line); border-left: 4px solid var(--navy); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); }
  .start-here-lead { margin: 4px 0 0; color: var(--muted); font-size: 13.5px; }
  .start-here-list { margin: 8px 0 0; padding-left: 22px; display: grid; gap: 6px; font-size: 14.5px; overflow-wrap: anywhere; }
  .start-here-resource { margin-left: 8px; color: var(--muted); font-size: 13px; overflow-wrap: anywhere; }
  .start-here-footer { margin: 12px 0 0; padding-top: 10px; border-top: 1px solid var(--line); font-weight: 700; font-size: 13.5px; }

  .top-risks-list { list-style: none; margin: 10px 0 0; padding: 0; display: grid; gap: 12px; }
  .risk-card { padding: 14px 16px; border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); }
  .risk-card.blocker { border-left-color: var(--red); }
  .risk-card.warning { border-left-color: var(--amber); }
  .risk-card.info { border-left-color: var(--blue); }
  .risk-card-head { display: flex; align-items: baseline; gap: 10px; }
  .risk-title { font-size: 16px; }
  .risk-card .rank { flex-shrink: 0; display: inline-grid; place-items: center; width: 20px; height: 20px; border-radius: 50%; background: var(--navy); color: white; font: 700 11px monospace; }
  .risk-card-chips { display: flex; flex-wrap: wrap; align-items: center; gap: 6px 10px; margin: 8px 0 0; }
  .rule-chip { display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px; border: 1px solid var(--line); border-radius: var(--radius-sm); background: #f0efe8; font-size: 11px; color: var(--muted); }
  .rule-chip .rule-id { padding: 0; background: none; font-size: 11px; color: var(--ink); }
  .risk-card .risk-resource { font-weight: 700; min-width: 0; overflow-wrap: anywhere; }
  .risk-card-section { margin-top: 12px; }
  .risk-card-section h4 { margin-bottom: 4px; }
  .risk-scan-detail { margin-top: 6px; }
  .risk-scan-detail summary { cursor: pointer; font-size: 12.5px; color: var(--blue); font-weight: 700; }
  .risk-scan-message { color: var(--muted); font-size: 13px; }

  /* Two-column desktop layout: card body (why/what-to-do) on the left,
     a slim action rail on the right so an operator can act ("view the
     full finding", "view its evidence", "copy the inspect command")
     without leaving the Summary tab. Stacks to one column on narrow
     viewports (see the max-width: 720px block below). */
  .risk-card-columns { display: grid; grid-template-columns: minmax(0, 1fr) 240px; gap: 18px; align-items: start; }
  .risk-card-body { min-width: 0; }
  .risk-card-rail { min-width: 0; padding: 12px 14px; border: 1px solid var(--line); border-radius: var(--radius); background: #f7f6f0; }
  .risk-card-rail h4 { margin-bottom: 2px; }
  .risk-card-rail .risk-body { font-size: 13.5px; }
  .risk-card-rail pre { margin-top: 6px; font-size: 12px; }
  .rail-btn { display: block; width: 100%; margin-top: 8px; padding: 7px 10px; border: 1px solid var(--line); background: white; color: var(--blue); font-size: 12px; font-weight: 700; text-align: center; cursor: pointer; border-radius: var(--radius-sm); }
  .rail-btn:hover { background: var(--blue-soft); }
  .rail-btn:focus-visible { outline: 2px solid var(--navy); outline-offset: 2px; }
  .rail-btn-nav { background: none; }

  /* Start Here's two-column layout mirrors the risk card's: fix order on
     the left, a lightweight "upgrade gate" self-check on the right. The
     checklist is plain browser-local UI state (unchecked on every load,
     nothing persisted or sent anywhere) — it exists so an operator can
     tick items off while working through the report, not as a record. */
  .start-here-columns { display: grid; grid-template-columns: minmax(0, 1fr) 220px; gap: 18px; align-items: start; margin-top: 4px; }
  .upgrade-gate { padding: 12px 14px; border: 1px solid var(--line); border-radius: var(--radius); background: #f7f6f0; }
  .upgrade-gate h4 { margin-bottom: 8px; }
  .upgrade-gate-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; font-size: 13px; }
  .upgrade-gate-list label { display: flex; align-items: flex-start; gap: 8px; cursor: pointer; }

  /* Brief flash so a reader can find the row a "View full finding"/"View
     evidence" jump landed on — fades back to the row's normal background
     via the transition below, not a hard cutoff. */
  .finding-row, table.appendix tr { transition: background-color .6s ease; }
  .finding-row.jump-highlight, table.appendix tr.jump-highlight, table.appendix tr.jump-highlight td { background: #fff3b0; }

  .section-subtitle { margin: -6px 0 10px; color: var(--muted); font-size: 13px; }
  .preview-actions-list { list-style: decimal; margin: 10px 0 0; padding-left: 22px; display: grid; gap: 10px; }
  .preview-actions-list li { padding: 10px 14px; border: 1px solid var(--line); border-left: 4px solid var(--line); border-radius: var(--radius); background: var(--surface); box-shadow: var(--shadow-card); font-size: 14px; }
  .preview-actions-list li.blocker { border-left-color: var(--red); }
  .preview-actions-list li.warning { border-left-color: var(--amber); }
  .preview-actions-list li.info { border-left-color: var(--blue); }
  .preview-action-head { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  .preview-actions-list .risk-resource { font-weight: 700; min-width: 0; overflow-wrap: anywhere; }
  .preview-action-command { margin: 4px 0 0; font-size: 12.5px; }
  .view-all-link { display: inline-block; margin-top: 8px; font-size: 13px; font-weight: 700; color: var(--blue); }
  /* Every rule's SafeFix.Command is a read-only kubectl get/describe (or
     aws ... describe-*) call, never an executable fix — this label makes
     that explicit everywhere the command appears, so it never reads like
     "run this and the problem is solved" when it's actually just gathering
     evidence before the human decides what to change. */
  .inspect-label { margin: 8px 0 2px; font-size: 11.5px; font-weight: 700; text-transform: uppercase; letter-spacing: .04em; color: var(--muted); }

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
  .severity-pill, .confidence-pill, .plane-pill, .rule-id, .priority-pill { display: inline-flex; align-items: center; white-space: nowrap; padding: 4px 8px; font-size: 10px; font-weight: 700; letter-spacing: .03em; }
  .severity-pill.blocker { background: var(--red-soft); color: #8e2d25; }
  .severity-pill.warning { background: var(--amber-soft); color: #754706; }
  .severity-pill.info { background: var(--blue-soft); color: var(--blue); }
  /* Priority is a separate axis from Severity (see internal/findings/priority.go)
     — P1 reuses the same red as a Blocker severity pill since a P1 is
     always also Blocker-severity, but P2-P4 get their own colors so a
     Blocker-severity P4 (e.g. an incompatible add-on) doesn't visually
     read as equally urgent as a P1 global blocker. */
  .priority-pill.p1 { background: var(--red-soft); color: #8e2d25; }
  .priority-pill.p2 { background: var(--amber-soft); color: #754706; }
  .priority-pill.p3 { background: var(--blue-soft); color: var(--blue); }
  .priority-pill.p4 { background: #f0efe8; color: var(--muted); }
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
  /* A grouped-plan step can embed a multi-command SafeFix.Command string
     (e.g. PDB-001's "get pdb ...\nget pods ...") — without pre-wrap, a
     plain <li>'s normal whitespace handling collapses that newline into a
     single run-on line with no separator, which reads as one concatenated
     command a user could paste verbatim. pre-wrap preserves the line break
     while still wrapping normally, matching how the "pre" rule above
     already treats every other multi-line command block in this report. */
  ol.grouped-plan li { margin: 2px 0; white-space: pre-wrap; word-break: break-word; }

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
  table.appendix td.key-evidence { min-width: 220px; max-width: 360px; }
  table.appendix td.key-evidence ul { margin: 0; padding-left: 16px; font-size: 12.5px; color: var(--muted); }
  table.appendix td.key-evidence li { overflow-wrap: anywhere; }
  table.appendix td.key-evidence li + li { margin-top: 3px; }

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
    .risk-card-columns, .start-here-columns, .upgrade-detail-body { grid-template-columns: 1fr; }
    .risk-card-rail { margin-top: 4px; }
  }
</style>
</head>
<body>
  <header class="banner" id="summary">
    <div class="banner-top-row">
      <p class="eyebrow">Upgrade readiness report</p>
	      <a id="console-link" href="/console/?findings=/findings.json#summary" class="console-link screen-only">Open Interactive Console</a>
    </div>
    <div class="decision-row">
      <div class="decision-mark {{.ResultClass}}"><span class="decision-label">{{.Decision}}</span></div>
      <div class="decision-copy">
        <h1 class="hero-title">{{.HeroTitle}}</h1>
        <p class="hero-subtext">{{.HeroSubtext}}</p>
        <div class="hero-badge-row">
          <span class="badge {{.ResultClass}}">{{.Result}}</span>
          <span class="hero-report-name">KubePreflight Scan Report</span>
        </div>
        <p class="why-line">{{.WhyLine}}</p>
        <p class="hero-explain">{{.HeroExplain}}</p>
        <p class="upgrade-context-line">{{.UpgradeLine}}</p>
        {{if .CurrentNote}}<p class="upgrade-context-note">{{.CurrentNote}}</p>{{end}}
      </div>
    </div>
    <dl class="banner-meta">
      <div class="meta-chip"><dt>Cluster</dt><dd>{{.Cluster}}</dd></div>
      <div class="meta-chip"><dt>Current version</dt><dd>{{.Current}}</dd></div>
      <div class="meta-chip"><dt>Target version</dt><dd>{{.Target}}</dd></div>
      <div class="meta-chip meta-chip-wide"><dt>Upgrade path</dt><dd>{{.UpgradePath}}{{if .UpgradeLabel}} <span class="meta-subtle">{{.UpgradeLabel}}</span>{{end}}</dd></div>
      <div class="meta-chip"><dt>Provider</dt><dd>{{.ProviderLabel}}</dd></div>
      <div class="meta-chip"><dt>AWS enrichment</dt><dd>{{.AWSEnrichmentLabel}}</dd></div>
      <div class="meta-chip"><dt>Scanned at</dt><dd>{{.ScannedAt}}</dd></div>
      {{if .NamespaceAllowlist}}<div class="meta-chip"><dt>Namespace allowlist</dt><dd>{{.NamespaceAllowlist}}</dd></div>{{end}}
      {{if .EKSCluster}}
      {{if .EKSCluster.Region}}<div class="meta-chip"><dt>Region</dt><dd>{{.EKSCluster.Region}}</dd></div>{{end}}
      {{if .EKSCluster.Version}}<div class="meta-chip"><dt>EKS version</dt><dd>{{.EKSCluster.Version}}</dd></div>{{end}}
      {{if .EKSCluster.PlatformVersion}}<div class="meta-chip"><dt>Platform version</dt><dd>{{.EKSCluster.PlatformVersion}}</dd></div>{{end}}
      {{if .EKSCluster.Status}}<div class="meta-chip"><dt>EKS status</dt><dd>{{.EKSCluster.Status}}</dd></div>{{end}}
      {{if .EKSCluster.SupportTypeLabel}}<div class="meta-chip"><dt>Support</dt><dd>{{.EKSCluster.SupportTypeLabel}}</dd></div>{{end}}
      {{if .EKSCluster.EndpointAccessLabel}}<div class="meta-chip"><dt>Endpoint access</dt><dd>{{.EKSCluster.EndpointAccessLabel}}</dd></div>{{end}}
      {{end}}
    </dl>
  </header>

  <section class="summary-grid" aria-label="Scan summary">
    {{if .Blockers}}<button type="button" class="metric metric-blocker metric-button" data-goto-severity="Blocker" aria-label="View blocker findings"><span>Blockers</span><strong>{{.Blockers}}</strong><small>View blocker findings</small></button>{{else}}<article class="metric metric-blocker" aria-disabled="true"><span>Blockers</span><strong>{{.Blockers}}</strong><small>No blockers found</small></article>{{end}}
    {{if .Warnings}}<button type="button" class="metric metric-warning metric-button" data-goto-severity="Warning" aria-label="View warning findings"><span>Warnings</span><strong>{{.Warnings}}</strong><small>View warning findings</small></button>{{else}}<article class="metric metric-warning" aria-disabled="true"><span>Warnings</span><strong>{{.Warnings}}</strong><small>No warnings found</small></article>{{end}}
    {{if .Infos}}<button type="button" class="metric metric-info metric-button" data-goto-severity="Info" aria-label="View info findings"><span>Info</span><strong>{{.Infos}}</strong><small>View info findings</small></button>{{else}}<article class="metric metric-info" aria-disabled="true"><span>Info</span><strong>{{.Infos}}</strong><small>No info findings</small></article>{{end}}
  </section>

  <nav class="tab-nav screen-only" role="tablist" aria-label="Report sections">
	    <button type="button" role="tab" aria-controls="summary-panel" aria-selected="true" class="tab-button tab-active" data-tab="summary">Summary</button>
	    <button type="button" role="tab" aria-controls="findings" aria-selected="false" class="tab-button" data-tab="findings">Findings<span class="tab-count">{{.TotalFindings}}</span></button>
	    <button type="button" role="tab" aria-controls="next-actions" aria-selected="false" class="tab-button" data-tab="actions">Next actions<span class="tab-count">{{len .NextActions}}</span></button>
	    <button type="button" role="tab" aria-controls="evidence-appendix" aria-selected="false" class="tab-button" data-tab="evidence">Evidence</button>
  </nav>

	  <div class="tab-panel" role="tabpanel" data-panel="summary" id="summary-panel">
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
	          <ul class="carry-forward-list">{{range .CarryForward}}<li><strong>{{.RuleID}}:</strong> {{.Reason}}{{if .Command}}<pre>{{.Command}}</pre>{{end}}</li>{{end}}</ul>
          {{end}}
        </li>
        {{end}}
      </ol>
	      <p class="upgrade-path-caption">Future-hop findings are projections. Items marked “Rescan required” are coverage requirements, not known findings. Re-run the shown command after each completed upgrade step.</p>
    </section>
    {{end}}

	    {{if .GlobalBlockerCount}}
    <section class="global-blocker-banner">
      <h2>Global API write blocker detected</h2>
      <p>This can block kubectl apply, kubectl patch, kubectl scale, Helm upgrades, and other remediation commands. Fix this before attempting other remediation.</p>
      <p class="global-blocker-count">{{.GlobalBlockerCount}} global blocker{{if ne .GlobalBlockerCount 1}}s{{end}} may prevent remediation commands from running.</p>
    </section>
	    {{end}}

	    {{if .CoverageIssues}}
	    <section class="global-blocker-banner">
	      <h2>Assessment incomplete</h2>
	      <p>One or more evidence sources could not be collected. Findings shown are valid, but absence of findings is not proof of readiness.</p>
	      {{range .CoverageIssues}}<h3>{{.Plane}}</h3><ul>{{range .Errors}}<li>{{.}}</li>{{end}}</ul>{{end}}
	    </section>
	    {{end}}

    {{if .UpgradeReadiness}}
    <section class="upgrade-readiness">
      <h2 class="section-title">Upgrade Readiness</h2>
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Verdict</th><th>Readiness score</th><th>Upgrade continue</th></tr>
        <tr>
          <td><span class="badge-{{if eq .UpgradeReadiness.Verdict "BLOCKED"}}blocked{{else if eq .UpgradeReadiness.Verdict "PASSED_WITH_WARNINGS"}}warn{{else if eq .UpgradeReadiness.Verdict "INCOMPLETE"}}warn{{else}}clean{{end}}">{{.UpgradeReadiness.Verdict}}</span></td>
          <td>{{.UpgradeReadiness.ReadinessScore}}/100</td>
          <td>{{yesNo .UpgradeReadiness.UpgradeContinue}}</td>
        </tr>
      </table>
      </div>
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Category</th><th>Status</th><th>Blockers</th><th>Warnings</th><th>Rule IDs</th></tr>
        {{range .UpgradeReadiness.Categories}}
        <tr>
          <td>{{.Name}}</td>
          <td><span class="badge-{{if eq .Status "Failed"}}blocked{{else if eq .Status "Warning"}}warn{{else}}clean{{end}}">{{.Status}}</span></td>
          <td>{{.BlockerCount}}</td>
          <td>{{.WarningCount}}</td>
          <td>{{range $i, $id := .RuleIDs}}{{if $i}} {{end}}<button type="button" class="rule-id-chip" data-goto-rule="{{$id}}">{{$id}}</button>{{end}}</td>
        </tr>
        {{end}}
      </table>
      </div>
    </section>
    {{end}}

    {{if .APICompatibility}}
    <section class="api-compatibility">
      <h2 class="section-title">Kubernetes API compatibility</h2>
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Status</th><th>Upgrade continue</th><th>Score impact</th><th>Removed objects</th><th>Deprecated objects</th><th>Critical impact</th></tr>
        <tr>
          <td><span class="badge-{{if eq .APICompatibility.Status "Failed"}}blocked{{else if eq .APICompatibility.Status "Warning"}}warn{{else}}clean{{end}}">{{.APICompatibility.Status}}</span></td>
          <td>{{yesNo .APICompatibility.UpgradeContinue}}</td>
          <td>{{.APICompatibility.ScoreImpact}}</td>
          <td>{{.APICompatibility.RemovedObjects}}</td>
          <td>{{.APICompatibility.DeprecatedObjects}}</td>
          <td>{{yesNo .APICompatibility.CriticalImpact}}</td>
        </tr>
      </table>
      </div>
      {{if .APICompatibility.RemovedFamilies}}
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Removed API version</th><th>Kind</th><th>Objects</th><th>Resources</th></tr>
        {{range .APICompatibility.RemovedFamilies}}
        <tr><td>{{.APIVersion}}</td><td>{{.Kind}}</td><td>{{.Count}}</td><td>{{range $i, $r := .Resources}}{{if $i}}, {{end}}{{$r}}{{end}}</td></tr>
        {{end}}
      </table>
      </div>
      {{end}}
      {{if .APICompatibility.DeprecatedFamilies}}
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Deprecated API version</th><th>Kind</th><th>Objects</th><th>Resources</th></tr>
        {{range .APICompatibility.DeprecatedFamilies}}
        <tr><td>{{.APIVersion}}</td><td>{{.Kind}}</td><td>{{.Count}}</td><td>{{range $i, $r := .Resources}}{{if $i}}, {{end}}{{$r}}{{end}}</td></tr>
        {{end}}
      </table>
      </div>
      {{end}}
    </section>
    {{end}}

    {{if .EKSAddons}}
    <section class="eks-addons">
      <h2 class="section-title">EKS add-ons</h2>
      <p class="section-subtitle">EKS does not automatically update add-ons after a Kubernetes minor version upgrade — review and update them explicitly. Add-ons that fail compatibility also appear as ADDON-001 findings below.</p>
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Add-on</th><th>Current version</th><th>Status</th><th>Compatible versions</th></tr>
        {{range .EKSAddons}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{if .CurrentVersion}}{{.CurrentVersion}}{{else}}—{{end}}</td>
          <td><span class="badge-{{.StatusClass}}">{{.StatusLabel}}</span></td>
          <td>{{if .CompatibleVersions}}{{.CompatibleVersions}}{{else}}—{{end}}</td>
        </tr>
        {{end}}
      </table>
      </div>
    </section>
    {{end}}

    {{if .ShowEKSNodegroups}}
    <section class="eks-nodegroups">
      <h2 class="section-title">EKS managed node groups</h2>
      <p class="section-subtitle">Inventory covers EKS managed node groups returned by AWS ListNodegroups. Self-managed nodes are not listed by that API.</p>
      {{if .EKSNodegroups}}
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Node group</th><th>Status</th><th>Version</th><th>Release</th><th>AMI type</th><th>Capacity</th><th>Desired / min / max</th><th>Update config</th><th>Health</th><th>Readiness</th></tr>
        {{range .EKSNodegroups}}
        <tr>
          <td>{{.Name}}</td>
          <td>{{.Status}}</td>
          <td>{{.Version}}</td>
          <td>{{.Release}}</td>
          <td>{{.AMIType}}</td>
          <td>{{.CapacityType}}</td>
          <td>{{.Scaling}}</td>
          <td>{{.UpdateConfig}}</td>
          <td>{{.Health}}</td>
          <td><span class="badge-{{.ReadinessClass}}">{{.Readiness}}</span></td>
        </tr>
        {{end}}
      </table>
      </div>
      {{else}}
      <p class="empty-state">No EKS managed node groups found. Self-managed nodes are not listed by the EKS ListNodegroups API.</p>
      {{end}}
    </section>
    {{end}}

    {{if .ShowEKSUpgradeInsights}}
    <section class="eks-upgrade-insights">
      <h2 class="section-title">EKS Upgrade Insights</h2>
      <p class="section-subtitle">AWS-native upgrade readiness checks from Amazon EKS. Insights may be up to 24 hours old; re-check after remediation.</p>
      {{if .EKSUpgradeInsightsUnavailable}}
      <p class="empty-state">EKS Upgrade Insights unavailable. Kubernetes findings are still valid.</p>
      {{else if .EKSUpgradeInsights}}
      <div class="table-wrap">
      <table class="appendix">
        <tr><th>Insight</th><th>Status</th><th>Kubernetes version</th><th>Last refreshed / transition</th><th>Recommendation</th><th>Details</th></tr>
        {{range .EKSUpgradeInsights}}
        <tr>
          <td>{{.Name}}</td>
          <td><span class="badge-{{.StatusClass}}">{{.Status}}</span></td>
          <td>{{.KubernetesVersion}}</td>
          <td>{{.LastRefreshed}}</td>
          <td>{{.Recommendation}}</td>
          <td>{{.Details}}</td>
        </tr>
        {{end}}
      </table>
      </div>
      {{else}}
      <p class="empty-state">No EKS upgrade insights returned.</p>
      {{end}}
    </section>
    {{end}}

    {{if .StartHere}}
    <section class="start-here">
      <h2 class="section-title">Start here</h2>
      <div class="start-here-columns">
        <div class="start-here-fixes">
          <p class="start-here-lead">Fix these in order:</p>
          <ol class="start-here-list">
            {{range .StartHere}}<li><strong>{{.Title}}</strong><span class="start-here-resource">{{.ResourceLabel}}</span></li>{{end}}
          </ol>
          <p class="start-here-footer">Do not start the upgrade until blockers = 0.</p>
        </div>
        {{if .Blockers}}
        <div class="upgrade-gate">
          <h4>Upgrade gate checklist</h4>
          <ul class="upgrade-gate-list">
            <li><label><input type="checkbox"> Blockers must be 0</label></li>
            <li><label><input type="checkbox"> Warnings reviewed</label></li>
            <li><label><input type="checkbox"> Evidence saved</label></li>
            <li><label><input type="checkbox"> Change window approved</label></li>
          </ul>
        </div>
        {{end}}
      </div>
    </section>
    {{end}}

    {{if .TopRisks}}
	    <section id="top-risks">
      <h2 class="section-title">Top risks</h2>
      <p class="section-subtitle priority-legend">Priority ranks upgrade urgency: P1 = fix now, P2 = fix before upgrade, P3 = fix before drain/maintenance, P4 = stabilize before starting.</p>
      <ol class="top-risks-list">
        {{range .TopRisks}}
        <li class="risk-card {{.SeverityClass}}">
          <div class="risk-card-columns">
            <div class="risk-card-body">
              <div class="risk-card-head">
                <span class="rank">{{.Rank}}</span>
                <h3 class="risk-title">{{.Title}}</h3>
              </div>
              <div class="risk-card-chips">
                <span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span>
                <span class="severity-pill {{.SeverityClass}}">{{severityActionLabel .Severity}}</span>
                <span class="rule-chip">Rule: <span class="rule-id">{{.RuleID}}</span></span>
                <span class="risk-resource">{{.ResourceLabel}}</span>
              </div>
              <div class="risk-card-section">
                <h4>Why this blocks upgrade</h4>
                <p class="risk-body">{{.Why}}</p>
                <details class="risk-scan-detail">
                  <summary>Show scan details</summary>
                  <p class="risk-body risk-scan-message">{{.Message}}</p>
                </details>
              </div>
              <div class="risk-card-section">
                <h4>What to do</h4>
                <p class="risk-body">{{.Remediation}}</p>
              </div>
            </div>
            <aside class="risk-card-rail" aria-label="Actions for {{.ResourceLabel}}">
              <h4>Next step</h4>
              <p class="risk-body">{{.NextStep}}</p>
              {{if .InspectCommand}}
              <p class="inspect-label">Inspect current state first. This does not change the cluster.</p>
              <pre id="{{.InspectElementID}}">{{.InspectCommand}}</pre>
              <button type="button" class="copy-btn rail-btn screen-only" data-copy-target="{{.InspectElementID}}">Copy inspect command</button>
              {{end}}
              <button type="button" class="rail-btn rail-btn-nav screen-only" data-goto-finding="{{.FindingTargetID}}" aria-label="View full finding for {{.ResourceLabel}}">View full finding</button>
              <button type="button" class="rail-btn rail-btn-nav screen-only" data-goto-evidence="{{.EvidenceTargetID}}" aria-label="View evidence for {{.ResourceLabel}}">View evidence</button>
            </aside>
          </div>
        </li>
        {{end}}
      </ol>
    </section>
    {{end}}

    {{if .UpgradeDetails}}
    <section class="upgrade-path-details">
      <h2 class="section-title">Upgrade path details</h2>
      <p class="section-subtitle">Advisory hop-by-hop context. Re-scan after each hop before treating the next hop as assessed.</p>
      <ol class="upgrade-details-list">
        {{range .UpgradeDetails}}
        <li class="upgrade-detail-card {{.StatusClass}}">
          <div class="upgrade-detail-head">
            <span class="hop-versions">{{.From}} &rarr; {{.To}}</span>
            <span class="badge-{{.StatusClass}}">{{.StatusLabel}}</span>
          </div>
          <div class="upgrade-detail-body">
            <h3>Assessment</h3>
            <p>{{.Assessment}}</p>
            <ul>{{range .FindingLines}}<li>{{.}}</li>{{end}}</ul>
          </div>
        </li>
        {{end}}
      </ol>
      <details class="upgrade-checks-details">
        <summary>Show checks to review</summary>
        <ul>{{range .UpgradeChecks}}<li>{{.}}</li>{{end}}</ul>
      </details>
    </section>
    {{end}}

    {{if .NextActionsPreview}}
    <section>
      <h2 class="section-title">Top next actions</h2>
      <p class="section-subtitle">Recommended fix order — worst first.</p>
      <ol class="preview-actions-list">
        {{range .NextActionsPreview}}
        <li class="{{.SeverityClass}}">
          <div class="preview-action-head">
            <span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span>
            <span class="severity-pill {{.SeverityClass}}">{{severityActionLabel .Severity}}</span>
            <span class="risk-resource">{{.ResourceLabel}}</span>
          </div>
          <p class="risk-body risk-reason">{{.Remediation}}</p>
          {{if .Command}}<p class="inspect-label">Inspect first — confirms current state, does not change anything:</p><pre class="preview-action-command">{{.Command}}</pre>{{end}}
        </li>
        {{end}}
      </ol>
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
          <div class="confidence-stat"><span>Provider: {{.ProviderLabel}}</span></div>
          <div class="confidence-stat"><span>AWS enrichment: {{.AWSEnrichmentLabel}}</span></div>
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

	  <div class="tab-panel hidden" role="tabpanel" data-panel="findings" id="findings">
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
      {{if .SafeFix.Command}}<p class="inspect-label">Inspect first — confirms current state, does not change anything:</p><pre id="{{$.ElementID}}-safefix">{{.SafeFix.Command}}</pre><button type="button" class="copy-btn" data-copy-target="{{$.ElementID}}-safefix">Copy command</button>{{end}}
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
    <details class="finding-row blocker" id="finding-{{.Fingerprint}}" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
      <summary>
        <span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span>
        <span class="rule-id">{{.RuleID}}</span>
        <span class="severity-pill blocker">{{.Severity}}</span>
        <span class="confidence-pill">{{.Confidence}}</span>
        {{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}
        {{if .GlobalBlocker}}<span class="global-blocker-badge">GLOBAL API WRITE BLOCKER</span>{{end}}
        <strong class="finding-resource">{{.ResourceLabel}}</strong>
        <span class="finding-message">{{.Message}}</span>
      </summary>
      <div class="finding-body">
        <div class="priority-detail {{priorityClass .Priority}}">
          <strong>Priority {{.Priority}}</strong>
          <p>{{.PriorityReason}}</p>
          <p class="priority-meta">Can upgrade continue: {{if .CanUpgradeContinue}}Yes{{else}}No{{end}} &middot; Affected scope: {{.AffectedScope}}</p>
        </div>
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
    <details class="finding-row warning" id="finding-{{.Fingerprint}}" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
      <summary>
        <span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span>
        <span class="rule-id">{{.RuleID}}</span>
        <span class="severity-pill warning">{{.Severity}}</span>
        <span class="confidence-pill">{{.Confidence}}</span>
        {{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}
        {{if .GlobalBlocker}}<span class="global-blocker-badge">GLOBAL API WRITE BLOCKER</span>{{end}}
        <strong class="finding-resource">{{.ResourceLabel}}</strong>
        <span class="finding-message">{{.Message}}</span>
      </summary>
      <div class="finding-body">
        <div class="priority-detail {{priorityClass .Priority}}">
          <strong>Priority {{.Priority}}</strong>
          <p>{{.PriorityReason}}</p>
          <p class="priority-meta">Can upgrade continue: {{if .CanUpgradeContinue}}Yes{{else}}No{{end}} &middot; Affected scope: {{.AffectedScope}}</p>
        </div>
        {{if .Evidence}}<h4>Evidence</h4><ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{end}}
        {{if .Remediation}}<div class="remediation-panel"><h4>Remediation</h4><pre id="{{.ElementID}}-remediation">{{.Remediation}}</pre><button type="button" class="copy-btn" data-copy-target="{{.ElementID}}-remediation">Copy remediation</button></div>{{end}}
        {{if .DependencyWarning}}<p class="dependency-warning">This command may fail until the admission webhook blocker is fixed.</p>{{end}}
        {{template "remediationDetail" .}}
      </div>
    </details>
    {{end}}
	    {{end}}

	    {{if .InfoFindings}}
	    <h2 class="section-title">Info ({{len .InfoFindings}})</h2>
	    {{range .InfoFindings}}
	    <details class="finding-row info" id="finding-{{.Fingerprint}}" data-finding="true" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
	      <summary><span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span><span class="rule-id">{{.RuleID}}</span><span class="severity-pill info">{{.Severity}}</span><span class="confidence-pill">{{.Confidence}}</span>{{if .PlaneLabel}}<span class="plane-pill">{{.PlaneLabel}}</span>{{end}}<strong class="finding-resource">{{.ResourceLabel}}</strong><span class="finding-message">{{.Message}}</span></summary>
	      <div class="finding-body">{{if .Evidence}}<h4>Evidence</h4><ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{end}}{{if .Remediation}}<div class="remediation-panel"><h4>Remediation</h4><pre id="{{.ElementID}}-remediation">{{.Remediation}}</pre><button type="button" class="copy-btn" data-copy-target="{{.ElementID}}-remediation">Copy remediation</button></div>{{end}}{{template "remediationDetail" .}}</div>
	    </details>
	    {{end}}
	    {{end}}
	  </div>

	  <div class="tab-panel hidden" role="tabpanel" data-panel="actions" id="next-actions">
    {{if .NextActions}}
    <h2 class="section-title">Next Actions ({{len .NextActions}})</h2>
    <ol class="next-actions">
    {{range .NextActions}}
      <li class="{{.SeverityClass}}" data-severity="{{.Severity}}" data-rule-ids="{{.RuleIDsJoined}}" data-resource="{{.ResourceLabel}}">
        <div class="action-head">
          <span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span>
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

	  <div class="tab-panel hidden" role="tabpanel" data-panel="evidence" id="evidence-appendix">
    {{if .AllFindings}}
    <h2 class="section-title">Evidence Appendix</h2>
	    <p>Every finding's resource identity, the concrete facts backing it, and its fingerprint — cross-reference by fingerprint for waivers/dedup.</p>
    <div class="table-wrap">
    <table class="appendix">
      <tr><th>Priority</th><th>Rule ID</th><th>Severity</th><th>Confidence</th><th>Resource</th><th>Key evidence</th><th>Fingerprint</th></tr>
      {{range .AllFindings}}
      <tr id="evidence-{{.Fingerprint}}" data-severity="{{.Severity}}" data-rule-ids="{{.RuleID}}" data-resource="{{.ResourceLabel}}">
        <td><span class="priority-pill {{priorityClass .Priority}}" title="{{.PriorityReason}}">{{.Priority}}</span></td><td>{{.RuleID}}</td><td>{{.Severity}}</td><td>{{.Confidence}}</td><td>{{.ResourceLabel}}</td><td class="key-evidence">{{if .Evidence}}<ul>{{range .Evidence}}<li>{{.}}</li>{{end}}</ul>{{else}}—{{end}}</td><td class="fingerprint">{{.Fingerprint}}</td>
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
	      tabButtons.forEach(function(btn) { var active = btn.getAttribute('data-tab') === name; btn.classList.toggle('tab-active', active); btn.setAttribute('aria-selected', String(active)); });
      tabPanels.forEach(function(panel) { panel.classList.toggle('hidden', panel.getAttribute('data-panel') !== name); });
    }

	    tabButtons.forEach(function(btn) {
	      btn.addEventListener('click', function() { activateTab(btn.getAttribute('data-tab')); });
	    });
	    tabButtons.forEach(function(btn, index) {
	      btn.addEventListener('keydown', function(event) {
	        if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
	        event.preventDefault();
	        var next = event.key === 'ArrowRight' ? (index + 1) % tabButtons.length : (index - 1 + tabButtons.length) % tabButtons.length;
	        tabButtons[next].focus(); activateTab(tabButtons[next].getAttribute('data-tab'));
	      });
	    });
	    var consoleLink = document.getElementById('console-link');
	    if (consoleLink && location.protocol === 'file:') consoleLink.hidden = true;
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

    // Top Risk cards' "View full finding"/"View evidence" buttons switch
    // tabs and scroll to the matching row, identified by fingerprint (see
    // FindingTargetID/EvidenceTargetID in html.go). Never executes a
    // command or changes report data — pure client-side navigation.
    function highlightAndScroll(el) {
      if (!el) return;
      el.scrollIntoView({ behavior: 'smooth', block: 'center' });
      el.classList.add('jump-highlight');
      setTimeout(function() { el.classList.remove('jump-highlight'); }, 2000);
    }

    document.querySelectorAll('[data-goto-finding]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var targetId = btn.getAttribute('data-goto-finding');
        if (!targetId) return;
        activateTab('findings');
        // Reset filters first so a jump target hidden by an active filter
        // is still reachable — an operator clicking this button expects to
        // see the finding, not an empty filtered-out row.
        sevBoxes.forEach(function(b) { b.checked = true; });
        ruleInput.value = '';
        resourceInput.value = '';
        apply();
        var el = document.getElementById(targetId);
        if (el && typeof el.setAttribute === 'function' && 'open' in el) { el.setAttribute('open', ''); }
        highlightAndScroll(el);
      });
    });

    document.querySelectorAll('[data-goto-severity]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var severity = btn.getAttribute('data-goto-severity');
        if (!severity) return;
        activateTab('findings');
        sevBoxes.forEach(function(b) { b.checked = b.value === severity; });
        ruleInput.value = '';
        resourceInput.value = '';
        apply();
        var target = document.querySelector('[data-finding][data-severity="' + severity + '"]');
        if (target && typeof target.setAttribute === 'function' && 'open' in target) { target.setAttribute('open', ''); }
        highlightAndScroll(target);
      });
    });

    document.querySelectorAll('[data-goto-rule]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var ruleId = btn.getAttribute('data-goto-rule');
        if (!ruleId) return;
        activateTab('findings');
        sevBoxes.forEach(function(b) { b.checked = true; });
        resourceInput.value = '';
        ruleInput.value = ruleId;
        apply();
        var target = document.querySelector('[data-finding][data-rule-ids]');
        Array.prototype.forEach.call(findingRows, function(row) {
          var ids = (row.getAttribute('data-rule-ids') || '').toLowerCase();
          if (ids.indexOf(ruleId.toLowerCase()) !== -1) { target = row; }
        });
        if (target && typeof target.setAttribute === 'function' && 'open' in target) { target.setAttribute('open', ''); }
        highlightAndScroll(target);
      });
    });

    document.querySelectorAll('[data-goto-evidence]').forEach(function(btn) {
      btn.addEventListener('click', function() {
        var targetId = btn.getAttribute('data-goto-evidence');
        if (!targetId) return;
        activateTab('evidence');
        highlightAndScroll(document.getElementById(targetId));
      });
    });

	    function fallbackCopy(text) {
	      var area = document.createElement('textarea'); area.value = text; area.style.position = 'fixed'; area.style.opacity = '0'; document.body.appendChild(area); area.select();
	      var copied = false; try { copied = document.execCommand('copy'); } catch (_) {} document.body.removeChild(area); return copied;
	    }
	    document.querySelectorAll('.copy-btn').forEach(function(btn) {
      var originalLabel = btn.textContent;
      btn.addEventListener('click', function(event) {
        event.preventDefault();
        var targetId = btn.getAttribute('data-copy-target');
        var pre = targetId ? document.getElementById(targetId) : btn.previousElementSibling;
        var text = pre ? pre.textContent : '';
        var reset = function() { setTimeout(function() { btn.textContent = originalLabel; }, 1500); };
	        if (navigator.clipboard && navigator.clipboard.writeText) {
	          navigator.clipboard.writeText(text).then(function() { btn.textContent = 'Copied'; reset(); }, function() { btn.textContent = fallbackCopy(text) ? 'Copied' : 'Copy unavailable'; reset(); });
	        } else {
	          btn.textContent = fallbackCopy(text) ? 'Copied' : 'Copy unavailable';
	          reset();
        }
      });
    });
  })();
  </script>
</body>
</html>
`

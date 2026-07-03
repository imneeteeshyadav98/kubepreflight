import { filterFindings, findingResourceLabel, parseFindingsDocument, resourceLabel, uniqueValues } from "./lib/findings-schema.mjs";

const state = { report: null, raw: null, selected: null, filters: { search: "", severity: "", confidence: "", namespace: "" } };
const $ = (selector) => document.querySelector(selector);
const elements = {
  importPanel: $("#import-panel"), workspace: $("#workspace"), error: $("#error-message"), exportButton: $("#export-button"),
  fileInputs: [$("#file-input"), $("#file-input-secondary")], loadDemo: $("#load-demo-button"), loadClean: $("#load-clean-button"),
  resultBadge: $("#result-badge"), cluster: $("#cluster-name"), subtitle: $("#scan-subtitle"), target: $("#target-version"), provider: $("#provider-name"), aws: $("#aws-enrichment"), scanned: $("#scanned-at"),
  assumptions: $("#assumptions"), assumptionList: $("#assumption-list"), confidenceList: $("#confidence-list"), findingsBody: $("#findings-body"), findingCount: $("#finding-count"), empty: $("#empty-state"), actions: $("#action-list"),
  search: $("#search-filter"), severity: $("#severity-filter"), confidence: $("#confidence-filter"), namespace: $("#namespace-filter"), reset: $("#reset-filters"),
  dialog: $("#finding-dialog"), dialogClose: $("#dialog-close"), dialogRule: $("#dialog-rule"), dialogTitle: $("#dialog-title"), dialogBadges: $("#dialog-badges"), dialogResources: $("#dialog-resources"), dialogEvidence: $("#dialog-evidence"), dialogRemediation: $("#dialog-remediation"), dialogFingerprint: $("#dialog-fingerprint"), copy: $("#copy-remediation"),
};

elements.fileInputs.forEach((input) => input.addEventListener("change", handleFile));
elements.loadDemo.addEventListener("click", loadDemo);
elements.loadClean.addEventListener("click", () => loadReport(cleanDemo(), "clean-demo.json"));
elements.exportButton.addEventListener("click", exportReport);
elements.dialogClose.addEventListener("click", () => elements.dialog.close());
elements.dialog.addEventListener("click", (event) => { if (event.target === elements.dialog) elements.dialog.close(); });
elements.copy.addEventListener("click", copyRemediation);
elements.reset.addEventListener("click", resetFilters);
[[elements.search, "search", "input"], [elements.severity, "severity", "change"], [elements.confidence, "confidence", "change"], [elements.namespace, "namespace", "change"]]
  .forEach(([element, key, event]) => element.addEventListener(event, () => { state.filters[key] = element.value; renderFindings(); }));

async function handleFile(event) {
  const file = event.target.files?.[0];
  if (!file) return;
  if (file.size > 10 * 1024 * 1024) return showError("File is larger than 10 MB. Use a scan-scoped findings.json.");
  try { loadReport(await file.text(), file.name); } catch (error) { showError(error.message); }
  event.target.value = "";
}

async function loadDemo() {
  try {
    const response = await fetch("../demo/sample-output/findings.json", { cache: "no-store" });
    if (!response.ok) throw new Error(`Demo returned HTTP ${response.status}`);
    loadReport(await response.text(), "demo/sample-output/findings.json");
  } catch (error) {
    showError(`Could not load the bundled demo. Serve the repository root, then open /web/: ${error.message}`);
  }
}

function loadReport(input, sourceName) {
  const report = parseFindingsDocument(input);
  state.report = report;
  state.raw = typeof input === "string" ? JSON.parse(input) : input;
  state.sourceName = sourceName;
  state.filters = { search: "", severity: "", confidence: "", namespace: "" };
  elements.search.value = ""; elements.severity.value = ""; elements.confidence.value = ""; elements.namespace.value = "";
  elements.error.hidden = true; elements.importPanel.hidden = true; elements.workspace.hidden = false; elements.exportButton.disabled = false;
  populateFilters(); renderDashboard(); renderFindings(); renderActions();
  location.hash = "summary";
}

function renderDashboard() {
  const { report } = state;
  elements.resultBadge.textContent = report.result;
  elements.resultBadge.className = `result-badge ${resultClass(report.result)}`;
  elements.cluster.textContent = report.clusterContext;
  elements.subtitle.textContent = `${report.findings.length} findings · source: ${state.sourceName}`;
  elements.target.textContent = report.targetVersion;
  elements.provider.textContent = report.provider;
  elements.aws.textContent = report.provider === "eks" || report.findings.some((finding) => finding.resources.some((resource) => resource.plane === "aws")) ? "true" : "false";
  elements.scanned.textContent = formatDate(report.scannedAt);
  $("#metric-result").textContent = report.result.replace("PASSED_WITH_WARNINGS", "WARNING");
  $("#metric-blockers").textContent = report.summary.blockers;
  $("#metric-warnings").textContent = report.summary.warnings;
  $("#metric-infos").textContent = report.summary.infos;

  const notes = [...report.assumptions];
  if (report.namespaceAllowlist.length) notes.push(`Namespace allowlist: ${report.namespaceAllowlist.join(", ")}`);
  elements.assumptions.hidden = notes.length === 0;
  replaceChildren(elements.assumptionList, notes.map((note) => node("li", {}, note)));

  const confidence = new Map();
  report.findings.forEach((finding) => confidence.set(finding.confidence, (confidence.get(finding.confidence) || 0) + 1));
  replaceChildren(elements.confidenceList, [...confidence.entries()].map(([name, count]) => node("div", { className: "confidence-stat" }, node("b", {}, count), node("span", {}, name))));
}

function populateFilters() {
  const findings = state.report.findings;
  setOptions(elements.severity, uniqueValues(findings, (finding) => [finding.severity]), "All severities");
  setOptions(elements.confidence, uniqueValues(findings, (finding) => [finding.confidence]), "All confidence");
  setOptions(elements.namespace, uniqueValues(findings, (finding) => finding.resources.map((resource) => resource.namespace || "cluster-scoped")), "All namespaces");
}

function renderFindings() {
  const findings = filterFindings(state.report.findings, state.filters);
  elements.findingCount.textContent = `${findings.length} of ${state.report.findings.length} findings`;
  elements.empty.hidden = findings.length !== 0;
  replaceChildren(elements.findingsBody, findings.map((finding) => {
    const primary = finding.resources[0];
    const row = node("tr", { tabIndex: 0, role: "button", ariaLabel: `Open ${finding.ruleId} details` },
      node("td", {}, severityPill(finding.severity)),
      node("td", {}, node("strong", {}, finding.ruleId)),
      node("td", { className: "resource-cell" }, node("strong", {}, findingResourceLabel(finding)), node("small", {}, finding.message)),
      node("td", {}, confidencePill(finding.confidence)),
      node("td", {}, node("span", { className: "plane-pill" }, [...new Set(finding.resources.map((resource) => resource.plane))].join(" + "))),
      node("td", {}, node("button", { className: "row-open", ariaLabel: "Open details" }, "→")),
    );
    row.addEventListener("click", () => openFinding(finding));
    row.addEventListener("keydown", (event) => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); openFinding(finding); } });
    row.dataset.namespace = primary.namespace || "cluster-scoped";
    return row;
  }));
}

function renderActions() {
  const severityRank = { Blocker: 0, Warning: 1, Info: 2 };
  const actions = [...state.report.findings].filter((finding) => finding.remediation).sort((a, b) => severityRank[a.severity] - severityRank[b.severity] || a.ruleId.localeCompare(b.ruleId));
  replaceChildren(elements.actions, actions.map((finding, index) => {
    const item = node("article", { className: "action-item" },
      node("span", { className: "action-number" }, String(index + 1).padStart(2, "0")),
      node("div", { className: "action-resource" }, node("strong", {}, findingResourceLabel(finding)), node("small", {}, `${finding.ruleId} · ${finding.severity}`)),
      node("p", { className: "action-copy" }, firstSentence(finding.remediation)),
    );
    item.addEventListener("click", () => openFinding(finding));
    return item;
  }));
}

function openFinding(finding) {
  state.selected = finding;
  elements.dialogRule.textContent = finding.ruleId;
  elements.dialogTitle.textContent = finding.message;
  replaceChildren(elements.dialogBadges, [severityPill(finding.severity), confidencePill(finding.confidence), node("span", { className: "plane-pill" }, [...new Set(finding.resources.map((resource) => resource.plane))].join(" + "))]);
  replaceChildren(elements.dialogResources, finding.resources.map((resource) => node("div", { className: "resource-card" },
    node("span", { className: "plane-pill" }, resource.plane),
    node("div", {}, node("strong", {}, resourceLabel(resource)), node("small", {}, resource.uid || resource.sourcePath || resource.providerId || "No occurrence ID")),
  )));
  replaceChildren(elements.dialogEvidence, finding.evidence.length ? finding.evidence.map((evidence) => node("li", {}, evidence)) : [node("li", {}, "No evidence supplied.")]);
  elements.dialogRemediation.textContent = finding.remediation;
  elements.dialogFingerprint.textContent = finding.fingerprint;
  elements.copy.textContent = "Copy steps";
  elements.dialog.showModal();
}

async function copyRemediation() {
  if (!state.selected) return;
  try { await navigator.clipboard.writeText(state.selected.remediation); elements.copy.textContent = "Copied"; }
  catch { elements.copy.textContent = "Copy unavailable"; }
}

function exportReport() {
  const blob = new Blob([JSON.stringify(state.raw, null, 2)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const anchor = node("a", { href: url, download: state.sourceName || "findings.json" });
  anchor.click(); URL.revokeObjectURL(url);
}

function resetFilters() {
  state.filters = { search: "", severity: "", confidence: "", namespace: "" };
  elements.search.value = ""; elements.severity.value = ""; elements.confidence.value = ""; elements.namespace.value = ""; renderFindings();
}

function setOptions(select, values, firstLabel) {
  replaceChildren(select, [node("option", { value: "" }, firstLabel), ...values.map((value) => node("option", { value }, value))]);
}

function severityPill(severity) { return node("span", { className: `severity-pill ${severity.toLowerCase()}` }, severity); }
function confidencePill(confidence) { return node("span", { className: "confidence-pill" }, confidence); }
function resultClass(result) { return result === "BLOCKED" ? "blocked" : result === "CLEAN" ? "clean" : "warning"; }
function formatDate(value) { if (!value) return "Not supplied"; const date = new Date(value); return Number.isNaN(date.valueOf()) ? value : date.toLocaleString(); }
function firstSentence(value) { const firstLine = value.split("\n").find((line) => line.trim()) || value; return firstLine.length > 240 ? `${firstLine.slice(0, 237)}…` : firstLine; }
function showError(message) { elements.error.textContent = message; elements.error.hidden = false; }
function replaceChildren(parent, children) { parent.replaceChildren(...children); }
function node(tag, attributes = {}, ...children) {
  const element = document.createElement(tag);
  Object.entries(attributes).forEach(([key, value]) => {
    if (key === "className") element.className = value;
    else if (key === "ariaLabel") element.setAttribute("aria-label", value);
    else if (key in element) element[key] = value;
    else element.setAttribute(key, value);
  });
  children.flat().forEach((child) => element.append(child instanceof Node ? child : document.createTextNode(String(child))));
  return element;
}

function cleanDemo() {
  return {
    targetVersion: "1.36", clusterContext: "payments-prod", provider: "eks", scannedAt: new Date().toISOString(), findings: [],
    summary: { blockers: 0, warnings: 0, infos: 0 }, assumptions: ["Local preview data — no cluster was contacted."],
  };
}

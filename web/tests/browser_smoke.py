#!/usr/bin/env python3
"""Functional browser smoke for the embedded KubePreflight Console.

The Console is a React app built once (`npm run build` in web/) and
embedded into the Go binary via go:embed (web/embed.go), served by
internal/reportserver at /console/ — the same code path `kubepreflight
scan` uses. This test drives the *real* server (via the consoledevserver
dev helper, cmd/consoledevserver — not part of the public CLI) rather than
a stand-in static file server, so it actually exercises the embedded-Console
mount, the printed ?findings= auto-load URL, and the sibling /report.html
and /findings.json routes together.

Requires Selenium, a local Chrome/Chromium installation, and `go` on PATH.
Run from anywhere; paths are resolved relative to the repository root.
"""

import json
import os
import subprocess
import tempfile
import time
from pathlib import Path

from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import Select, WebDriverWait


def click_severity_chip(driver, severity):
    # The chip's checkbox input is visually hidden (position: absolute;
    # opacity: 0) in favor of the styled <label> — a plain .click()
    # intercepts on the hidden input, so click the label instead (same
    # element a real user would click), via JS to sidestep any residual
    # scroll/overlap flakiness.
    label = driver.find_element(By.CSS_SELECTOR, f"label.chip-{severity.lower()}")
    driver.execute_script("arguments[0].scrollIntoView({block:'center'}); arguments[0].click();", label)


def click_tab(driver, name):
    # Tab buttons render as e.g. "Findings11" (label + count badge in the
    # same element), so match on the label being a text prefix rather than
    # an exact string.
    button = driver.find_element(By.XPATH, f'//button[@role="tab"][contains(@class,"tab-button")][starts-with(normalize-space(.), "{name}")]')
    driver.execute_script("arguments[0].click();", button)


ROOT = Path(__file__).resolve().parents[2]


def wait(driver, predicate, message):
    WebDriverWait(driver, 10).until(predicate, message=message)


def visible_rows(driver):
    return [row for row in driver.find_elements(By.CSS_SELECTOR, "#findings-body tr") if row.is_displayed()]


def upload(driver, path):
    driver.find_element(By.ID, "file-input").send_keys(str(path))
    wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), "workspace did not appear after upload")


def write_report(path, severity):
    finding = {
        "ruleId": "SMOKE-001",
        "severity": severity,
        "confidence": "STATIC_CERTAIN",
        "message": f"Synthetic {severity.lower()} browser smoke finding",
        "resources": [{"plane": "live", "kind": "Deployment", "scope": "namespaced", "namespace": "smoke", "name": "api"}],
        "evidence": ["browser smoke evidence"],
        "remediation": "Browser smoke remediation.",
        "fingerprint": f"smoke-{severity.lower()}",
    }
    path.write_text(json.dumps({"targetVersion": "1.36", "clusterContext": "smoke", "findings": [finding]}), encoding="utf-8")


def write_large_report(path, count=1000):
    findings = []
    for i in range(count):
        findings.append({
            "ruleId": "PDB-001" if i % 2 == 0 else "WH-001",
            "severity": "Blocker" if i % 5 == 0 else "Warning",
            "confidence": "STATIC_CERTAIN",
            "message": f"Synthetic large report finding workload-{i}",
            "resources": [{"plane": "live", "kind": "Deployment", "scope": "namespaced", "namespace": f"ns-{i % 20}", "name": f"workload-{i}"}],
            "evidence": [f"index: {i}"],
            "remediation": "Review workload configuration.",
            "fingerprint": f"large-fp-{i}",
        })
    path.write_text(json.dumps({"targetVersion": "1.36", "clusterContext": "large-smoke", "findings": findings}), encoding="utf-8")


class ReportServer:
    """Starts the real internal/reportserver against a fixture directory
    by building and running cmd/consoledevserver, parsing its printed
    URLs.

    synthetic=True ignores output_dir/findings_name entirely and instead
    passes --synthetic, which makes consoledevserver render a fresh
    findings.json/report.html straight from internal/report (see
    writeSyntheticFixture in cmd/consoledevserver/main.go) — no cluster, no
    committed fixture that can go stale, no dependency on a real cluster's
    live state. This is what every check in this file uses now; demo
    output is no longer committed to the repo at all (see demo/README.md —
    it's generated locally, on demand). The output_dir/findings_name path
    still works generically for pointing this at any real fixture
    directory you happen to have on disk, but nothing here relies on it."""

    def __init__(self, output_dir, findings_name="findings.json", synthetic=False):
        self.output_dir = output_dir
        self.findings_name = findings_name
        self.synthetic = synthetic
        self.process = None
        self.report_url = None
        self.console_url = None
        self._binary = None
        self._owns_binary = False

    def __enter__(self):
        # `go run` spawns the compiled binary as a child process and does
        # not forward SIGTERM to it, so terminating the `go run` wrapper
        # orphans the actual server — confirmed by leftover
        # consoledevserver processes surviving past this script's exit.
        # Building once and exec'ing the binary directly gives us a single
        # process to manage.
        self._binary = os.environ.get("KUBEPREFLIGHT_CONSOLEDEVSERVER")
        if not self._binary:
            binary_dir = tempfile.mkdtemp(prefix="kubepreflight-consoledevserver-")
            self._binary = str(Path(binary_dir) / "consoledevserver")
            self._owns_binary = True
            subprocess.run(["go", "build", "-o", self._binary, "./cmd/consoledevserver"], cwd=ROOT, check=True)

        if self.synthetic:
            args = [self._binary, "--synthetic"]
        else:
            args = [self._binary, "--dir", str(self.output_dir), "--findings", self.findings_name]
        self.process = subprocess.Popen(
            args,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        deadline = time.time() + 10
        while time.time() < deadline:
            line = self.process.stdout.readline()
            if not line:
                if self.process.poll() is not None:
                    raise RuntimeError("consoledevserver exited before printing its URLs")
                continue
            line = line.strip()
            if line.startswith("REPORT "):
                self.report_url = line.removeprefix("REPORT ")
            elif line.startswith("CONSOLE "):
                self.console_url = line.removeprefix("CONSOLE ")
            if self.report_url and self.console_url:
                return self
        raise RuntimeError("timed out waiting for consoledevserver to start")

    def __exit__(self, *_exc):
        if self.process:
            self.process.terminate()
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.process.kill()
        if self._binary and self._owns_binary:
            Path(self._binary).unlink(missing_ok=True)


def main():
    def progress(stage):
        print(f"browser smoke: {stage}", flush=True)

    with tempfile.TemporaryDirectory(prefix="kubepreflight-console-") as temp:
        temp_path = Path(temp)
        options = webdriver.ChromeOptions()
        options.add_argument("--headless=new")
        options.add_argument("--no-sandbox")
        options.add_argument("--disable-dev-shm-usage")
        options.add_argument("--window-size=1440,1100")
        options.add_experimental_option("prefs", {
            "download.default_directory": str(temp_path),
            "download.prompt_for_download": False,
        })

        with ReportServer(None, synthetic=True) as server:
            driver = webdriver.Chrome(options=options)
            try:
                # Auto-load: the exact URL `kubepreflight scan` prints after
                # a scan completes must render the dashboard immediately,
                # with no blank import screen in between.
                driver.get(server.console_url)
                assert driver.title == "KubePreflight Console"
                wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), "?findings= did not auto-load the synthetic report")
                progress("desktop Console loaded")
                assert driver.find_element(By.ID, "result-badge").text == "BLOCKED"
                # Counts reflect writeSyntheticFixture (cmd/consoledevserver/
                # main.go) exactly: 4 Blocker (PDB-002, PDB-001, API-001,
                # WH-002), 1 Warning (NODE-003), 1 Info (EKS-NG-003).
                assert driver.find_element(By.ID, "metric-blockers").text == "4"
                assert driver.find_element(By.ID, "metric-warnings").text == "1"
                # React unmounts the import panel entirely once a report is
                # loaded (unlike the old vanilla-JS Console, which toggled a
                # `hidden` attribute on an always-present element).
                assert len(driver.find_elements(By.ID, "import-panel")) == 0

                # Decision strip: GO/REVIEW/NO-GO framing, above the fold,
                # part of the always-visible chrome (not inside a tab).
                assert driver.find_element(By.CSS_SELECTOR, ".decision-chip").text == "NO-GO"
                assert "blocker" in driver.find_element(By.ID, "decision-why").text.lower()

                # Single-page command center: lands on Summary, and only
                # one tab's content is present in the DOM at a time.
                assert driver.find_element(By.CSS_SELECTOR, '.tab-button[data-tab="summary"]').get_attribute("aria-selected") == "true"
                assert len(driver.find_elements(By.ID, "findings-body")) == 0, "Findings table should not be mounted on the Summary tab"

                # Top risks: highest-severity findings surfaced in the
                # Summary tab preview; clicking one navigates to the
                # Findings tab with that finding selected inline (no modal).
                top_risks = driver.find_element(By.ID, "top-risks")
                assert top_risks.find_elements(By.CSS_SELECTOR, ".top-risk-card")
                top_risks.find_elements(By.CSS_SELECTOR, ".top-risk-card")[0].click()
                wait(driver, lambda d: d.find_element(By.CSS_SELECTOR, '.tab-button[data-tab="findings"]').get_attribute("aria-selected") == "true", "clicking a top risk did not switch to the Findings tab")
                wait(driver, lambda d: d.find_elements(By.ID, "finding-detail"), "top risk card did not select a finding")

                # Next actions: its own tab, grouped by severity, each item
                # has its own copy button.
                click_tab(driver, "Next Actions")
                actions = driver.find_element(By.ID, "actions")
                # .action-group-title is CSS text-transform: uppercase, so
                # Selenium's rendered .text reflects that transform.
                # Actions are grouped by conceptual resource, so their
                # counts are intentionally lower than raw finding counts —
                # PDB-001 and PDB-002 both target preflight-lab/
                # critical-app-pdb and merge into one Blocker group, so 6
                # raw findings become 5 groups: 3 Blocker, 1 Warning, 1 Info.
                assert "BLOCKERS (3)" in actions.text
                assert "WARNINGS (1)" in actions.text
                actions.find_elements(By.CSS_SELECTOR, ".action-copy-button")[0].click()

                # Regression guard: a grouped action's related-findings list
                # (.evidence-list) is a 5th CSS-grid child beyond
                # .action-item's 4 explicit columns (number/resource/copy/
                # button) — without an explicit grid-column, it silently
                # auto-placed into the narrow 36px number column instead of
                # spanning the card, which read as squeezed/unreadable
                # multi-line prose in a live demo review. Only real-browser
                # layout (not jsdom) can catch this, hence this smoke test
                # rather than a Vitest unit test.
                grouped_items = [
                    item for item in actions.find_elements(By.CSS_SELECTOR, ".action-item")
                    if item.find_elements(By.CSS_SELECTOR, ".evidence-list")
                ]
                assert grouped_items, "expected at least one grouped Next Action with a related-findings list in the synthetic fixture"
                item_width = grouped_items[0].size["width"]
                evidence_width = grouped_items[0].find_element(By.CSS_SELECTOR, ".evidence-list").size["width"]
                assert evidence_width > item_width * 0.8, (
                    f"related-findings list width ({evidence_width}px) is squeezed relative to its "
                    f"action-item ({item_width}px) — the .evidence-list grid-column placement regressed"
                )

                # Findings tab: severity chips, confidence, namespace, and
                # text filters, plus the split-pane list + detail. Counts
                # below reflect writeSyntheticFixture's 6 findings exactly
                # (see the metric-blockers/metric-warnings note above).
                click_tab(driver, "Findings")
                wait(driver, lambda d: len(visible_rows(d)) == 6, "findings tab did not render the full table")
                assert driver.find_element(By.ID, "finding-count").text == "6 of 6 findings"

                # Chips start all-selected; clicking "Blocker" deselects it,
                # leaving Warning+Info visible (1 warning, 1 info here) —
                # not a "Blocker only" filter (see toggleSeverity).
                click_severity_chip(driver, "Blocker")
                wait(driver, lambda d: len(visible_rows(d)) == 2, "severity chip filter failed")
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                Select(driver.find_element(By.ID, "confidence-filter")).select_by_visible_text("STATIC_CERTAIN")
                assert len(visible_rows(driver)) == 2  # API-001, NODE-003
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                Select(driver.find_element(By.ID, "namespace-filter")).select_by_visible_text("kube-system")
                assert len(visible_rows(driver)) == 1  # NODE-003
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                driver.find_element(By.ID, "search-filter").send_keys("WH-002")
                wait(driver, lambda d: len(visible_rows(d)) == 1, "search filter failed")
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))

                # Deselecting every severity chip shows zero findings, not
                # every finding — matches report.html's checkbox semantics.
                for severity in ("Blocker", "Warning", "Info"):
                    click_severity_chip(driver, severity)
                wait(driver, lambda d: len(visible_rows(d)) == 0, "deselecting every chip did not show zero findings")
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                wait(driver, lambda d: len(visible_rows(d)) == 6, "clear filters did not restore every chip")

                # The highest-severity visible finding is selected as soon
                # as the tab opens. Selecting another row still updates the
                # inline detail pane — including both copy actions.
                wait(driver, lambda d: d.find_elements(By.ID, "finding-detail"), "findings tab did not auto-select a finding")
                visible_rows(driver)[0].click()
                wait(driver, lambda d: d.find_elements(By.ID, "finding-detail"), "detail pane did not populate")
                assert driver.find_element(By.ID, "dialog-evidence").text
                driver.find_element(By.ID, "copy-remediation").click()
                wait(driver, lambda d: d.find_element(By.ID, "copy-remediation").text in {"Copied", "Copy unavailable"}, "copy action did not resolve")
                driver.find_element(By.ID, "copy-finding-json").click()
                wait(driver, lambda d: d.find_element(By.ID, "copy-finding-json").text in {"Copied", "Copy unavailable"}, "copy finding JSON did not resolve")

                # Export produces a local JSON download.
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "export-button"))
                wait(driver, lambda _d: any(temp_path.glob("*.json")), "export did not download JSON")

                # Large report path: keep the initial DOM bounded while
                # counts and search still operate over the complete report.
                large_fixture = temp_path / "large.json"
                write_large_report(large_fixture)
                upload(driver, large_fixture)
                click_tab(driver, "Findings")
                wait(driver, lambda d: len(visible_rows(d)) == 250, "large findings tab did not cap the initial row count")
                assert driver.find_element(By.ID, "finding-count").text == "1000 of 1000 findings"
                assert "Showing 250 of 1000" in driver.find_element(By.CSS_SELECTOR, ".pagination-controls").text
                driver.find_element(By.ID, "search-filter").send_keys("workload-999")
                wait(driver, lambda d: len(visible_rows(d)) == 1, "large findings search did not search the full report")
                assert "workload-999" in visible_rows(driver)[0].text
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))

                # Regression guard for the tablet/short-viewport bug:
                # reports with an unknown current version render a taller
                # decision strip, and at 768px wide the two-column metric
                # grid used to consume enough fixed dashboard height that
                # the Findings list flexed down to 0px. The list must keep
                # a usable scroll area instead.
                driver.set_window_size(768, 500)
                click_tab(driver, "Findings")
                wait(driver, lambda d: len(d.find_elements(By.CSS_SELECTOR, "#findings-body tr")) == 250, "tablet short viewport did not mount findings rows")
                list_pane = driver.find_element(By.CSS_SELECTOR, ".findings-list-pane")
                list_scroll = driver.find_element(By.CSS_SELECTOR, ".findings-list-scroll")
                first_row = driver.find_elements(By.CSS_SELECTOR, "#findings-body tr")[0]
                assert list_pane.size["height"] > 0, "tablet short viewport collapsed the findings list pane"
                assert list_scroll.size["height"] >= 120, f"tablet short viewport findings scroller too short: {list_scroll.size['height']}px"
                assert first_row.is_displayed(), "tablet short viewport did not show the first finding row"
                assert list_scroll.get_property("scrollHeight") > list_scroll.get_property("clientHeight"), "large findings list should remain scrollable"

                # Manual "Import findings.json" still overrides whatever was
                # auto-loaded — upload a different, synthetic report and
                # confirm the workspace switches to it (and back to the
                # Summary tab).
                warning_fixture = temp_path / "warning.json"
                write_report(warning_fixture, "Warning")
                upload(driver, warning_fixture)
                assert driver.find_element(By.ID, "result-badge").text == "PASSED_WITH_WARNINGS"
                assert driver.find_element(By.ID, "metric-blockers").text == "0"
                assert driver.find_element(By.CSS_SELECTOR, '.tab-button[data-tab="summary"]').get_attribute("aria-selected") == "true"

                # Malformed JSON is rejected safely — via the header's
                # always-present "Import findings.json" input, since the
                # real production server always has a findings.json (Start()
                # requires it), so the blank import panel with its own
                # "Choose findings.json"/demo/clean buttons is structurally
                # unreachable through this server by design once a scan has
                # run. Those import-panel-only affordances (bundled demo,
                # clean preview) share the same loadReport() code path this
                # test already exercises via auto-load and manual import, and
                # are covered directly by the Vitest component tests
                # (web/src/App.test.tsx), which can render Console before
                # any report is loaded.
                malformed = temp_path / "malformed.json"
                malformed.write_text("{ definitely not json", encoding="utf-8")
                driver.find_element(By.ID, "file-input").send_keys(str(malformed))
                wait(driver, lambda d: d.find_element(By.ID, "error-message").is_displayed(), "malformed JSON error was not shown")
                assert "Invalid JSON" in driver.find_element(By.ID, "error-message").text

                # An explicit ?findings= path that 404s must show a readable
                # error rather than silently staying on the blank panel.
                driver.get(server.console_url.split("?")[0] + "?findings=/does-not-exist.json")
                wait(driver, lambda d: d.find_element(By.ID, "error-message").is_displayed(), "missing ?findings= target did not surface an error")
                assert "does-not-exist.json" in driver.find_element(By.ID, "error-message").text

                # Mobile layout: the app shell degrades to a normal
                # scrolling document (see the max-width:720px block in
                # styles.css) rather than trying to keep the fixed-viewport
                # split panes, which don't fit a phone screen.
                driver.set_window_size(390, 844)
                driver.get(server.console_url)
                wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), "mobile auto-load did not render the workspace")
                driver.execute_script("window.scrollTo(1000, 0)")
                assert driver.execute_script("return window.scrollX") == 0, "mobile document scrolls horizontally"
                click_tab(driver, "Findings")
                visible_rows(driver)[0].click()
                wait(driver, lambda d: d.find_elements(By.ID, "finding-detail"), "mobile: selecting a row did not show detail")
                assert not driver.find_element(By.CSS_SELECTOR, ".findings-list-pane").is_displayed(), "mobile: list pane should hide once a finding is selected"
                driver.find_element(By.CSS_SELECTOR, ".back-to-list").click()
                wait(driver, lambda d: d.find_element(By.CSS_SELECTOR, ".findings-list-pane").is_displayed(), "mobile: back-to-list did not restore the list pane")
                driver.save_screenshot(str(temp_path / "console-mobile.png"))
                progress("Console interactions and mobile layout passed")

                # The sibling report.html route shares the same command-
                # center pass (decision framing, Top Risks, tabs, collapsed
                # finding rows) — quick regression check on the same server.
                driver.get(server.report_url)
                assert "KubePreflight Scan Report" in driver.page_source
                assert driver.find_element(By.CSS_SELECTOR, ".decision-label").text == "NO-GO"
                assert driver.find_elements(By.CSS_SELECTOR, "[data-panel='summary'] .top-risks-list li")
                # Only the Summary tab panel is visible by default.
                assert driver.find_element(By.CSS_SELECTOR, '[data-panel="findings"]').get_attribute("class").find("hidden") != -1
                driver.find_element(By.CSS_SELECTOR, '.tab-button[data-tab="findings"]').click()
                wait(driver, lambda d: "hidden" not in d.find_element(By.CSS_SELECTOR, '[data-panel="findings"]').get_attribute("class"), "report.html Findings tab did not become visible")
                finding_rows = driver.find_elements(By.CSS_SELECTOR, ".finding-row")
                assert finding_rows
                assert all(row.get_attribute("open") is None for row in finding_rows), "report.html finding rows must be collapsed by default"
                progress("static report interactions passed")

                # Permanent regression guard: no page should ever gain a
                # horizontal scrollbar. This is the exact class of bug found
                # and fixed several times during the width/layout polish
                # pass — a long resource name, shell command, or fingerprint
                # that doesn't wrap can silently widen the whole page.
                # Checked at a phone, a laptop, and a large desktop width,
                # across every report.html tab plus the Console's default
                # view.
                def assert_no_horizontal_overflow(d, label):
                    scroll_width = d.execute_script("return document.documentElement.scrollWidth")
                    inner_width = d.execute_script("return window.innerWidth")
                    assert scroll_width <= inner_width + 1, (
                        f"{label}: horizontal overflow (scrollWidth={scroll_width} > innerWidth={inner_width})"
                    )

                widths = ((390, 844), (1366, 900), (1920, 1080))

                # Uses the synthetic, cluster-independent fixture (fresh
                # findings.json/report.html rendered straight from
                # internal/report) so this guard always reflects the
                # current template/CSS rather than a frozen capture.
                with ReportServer(None, synthetic=True) as synth_server:
                    progress("synthetic overflow fixture loaded")
                    for width, height in widths:
                        driver.set_window_size(width, height)
                        driver.get(synth_server.report_url)
                        assert_no_horizontal_overflow(driver, f"report.html summary @ {width}px")
                        for tab in ("findings", "actions", "evidence"):
                            driver.find_element(By.CSS_SELECTOR, f'.tab-button[data-tab="{tab}"]').click()
                            assert_no_horizontal_overflow(driver, f"report.html {tab} @ {width}px")

                    # Top Risk card action rail: "View full finding"/"View
                    # evidence" must switch tabs, scroll to, and highlight
                    # the exact matching row (identified by fingerprint) —
                    # never execute a command, never leave the target
                    # unopened/unreachable. Exercises the real click path,
                    # not just the HTML output, since this behavior only
                    # exists once JS runs. Uses the synthetic fixture so
                    # this always reflects the current template/JS.
                    driver.set_window_size(1366, 900)
                    driver.get(synth_server.report_url)
                    # writeSyntheticFixture has all three severities present
                    # (4 Blocker, 1 Warning, 1 Info) — every metric card
                    # should be clickable. (The disabled/zero-count card
                    # rendering — aria-disabled on a non-button <article> —
                    # isn't exercised here since every severity is non-zero
                    # in this fixture by design; that CSS path is a static
                    # template branch already covered by internal/report's
                    # own Go tests.)
                    cards = {}
                    for severity in ("Blocker", "Warning", "Info"):
                        card = driver.find_element(By.CSS_SELECTOR, f'[data-goto-severity="{severity}"]')
                        assert card.tag_name == "button", f"non-zero {severity} card should be a button"
                        cards[severity] = card
                    driver.execute_script("arguments[0].click();", cards["Blocker"])
                    wait(driver, lambda d: "hidden" not in d.find_element(By.CSS_SELECTOR, '[data-panel="findings"]').get_attribute("class"), "Blockers summary card did not switch to Findings")
                    severity_boxes = driver.find_elements(By.CSS_SELECTOR, ".sev-filter")
                    assert {box.get_attribute("value"): box.is_selected() for box in severity_boxes} == {"Blocker": True, "Warning": False, "Info": False}
                    blocker_target = driver.find_element(By.CSS_SELECTOR, '[data-finding][data-severity="Blocker"]')
                    wait(driver, lambda d: blocker_target.get_attribute("open") is not None, "Blockers summary card did not expand the first blocker")
                    wait(driver, lambda d: "jump-highlight" in blocker_target.get_attribute("class"), "Blockers summary card did not highlight the first blocker")

                    driver.get(synth_server.report_url)
                    rail = driver.find_element(By.CSS_SELECTOR, ".risk-card-rail")
                    assert "next step" in rail.text.lower(), "Top Risk card action rail missing its Next step summary"

                    view_finding_btn = driver.find_element(By.CSS_SELECTOR, "[data-goto-finding]")
                    finding_target_id = view_finding_btn.get_attribute("data-goto-finding")
                    driver.execute_script("arguments[0].click();", view_finding_btn)
                    wait(driver, lambda d: "hidden" not in d.find_element(By.CSS_SELECTOR, '[data-panel="findings"]').get_attribute("class"), "View full finding did not switch to the Findings tab")
                    finding_target = driver.find_element(By.ID, finding_target_id)
                    wait(driver, lambda d: finding_target.get_attribute("open") is not None, "View full finding did not expand the matching finding row")
                    wait(driver, lambda d: "jump-highlight" in finding_target.get_attribute("class"), "View full finding did not highlight the matching finding row")

                    driver.get(synth_server.report_url)
                    view_evidence_btn = driver.find_element(By.CSS_SELECTOR, "[data-goto-evidence]")
                    evidence_target_id = view_evidence_btn.get_attribute("data-goto-evidence")
                    driver.execute_script("arguments[0].click();", view_evidence_btn)
                    wait(driver, lambda d: "hidden" not in d.find_element(By.CSS_SELECTOR, '[data-panel="evidence"]').get_attribute("class"), "View evidence did not switch to the Evidence tab")
                    evidence_target = driver.find_element(By.ID, evidence_target_id)
                    wait(driver, lambda d: "jump-highlight" in evidence_target.get_attribute("class"), "View evidence did not highlight the matching evidence row")
                    progress("Top Risk card action rail navigation passed")

                    for width, height in widths:
                        driver.set_window_size(width, height)
                        driver.get(synth_server.console_url)
                        wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), f"console did not auto-load at {width}px")
                        assert_no_horizontal_overflow(driver, f"console summary @ {width}px")
                        click_tab(driver, "Findings")
                        wait(driver, lambda d: d.find_elements(By.ID, "finding-detail"), f"console did not auto-select a finding at {width}px")
                        assert_no_horizontal_overflow(driver, f"console findings @ {width}px")
                        list_scroll = driver.find_element(By.CSS_SELECTOR, ".findings-list-scroll")
                        assert list_scroll.get_property("scrollWidth") <= list_scroll.get_property("clientWidth") + 1, (
                            f"console findings list has a horizontal scrollbar at {width}px"
                        )

                progress("PASS")
            finally:
                driver.quit()


if __name__ == "__main__":
    main()

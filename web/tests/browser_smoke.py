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
DEMO_FIXTURES = ROOT / "demo" / "sample-output"


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


class ReportServer:
    """Starts the real internal/reportserver against a fixture directory
    by building and running cmd/consoledevserver, parsing its printed
    URLs.

    synthetic=True ignores output_dir/findings_name entirely and instead
    passes --synthetic, which makes consoledevserver render a fresh
    findings.json/report.html straight from internal/report (see
    writeSyntheticFixture in cmd/consoledevserver/main.go) — no cluster, no
    committed fixture that can go stale. Use this for checks that must
    always reflect the *current* template/CSS (like the horizontal-overflow
    guard below); use a real fixture directory for everything else, since
    demo/sample-output/ is deliberately a captured real-world example."""

    def __init__(self, output_dir, findings_name="findings.json", synthetic=False):
        self.output_dir = output_dir
        self.findings_name = findings_name
        self.synthetic = synthetic
        self.process = None
        self.report_url = None
        self.console_url = None
        self._binary = None

    def __enter__(self):
        # `go run` spawns the compiled binary as a child process and does
        # not forward SIGTERM to it, so terminating the `go run` wrapper
        # orphans the actual server — confirmed by leftover
        # consoledevserver processes surviving past this script's exit.
        # Building once and exec'ing the binary directly gives us a single
        # process to manage.
        binary_dir = tempfile.mkdtemp(prefix="kubepreflight-consoledevserver-")
        self._binary = str(Path(binary_dir) / "consoledevserver")
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
        if self._binary:
            Path(self._binary).unlink(missing_ok=True)


def main():
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

        with ReportServer(DEMO_FIXTURES) as server:
            driver = webdriver.Chrome(options=options)
            try:
                # Auto-load: the exact URL `kubepreflight scan` prints after
                # a scan completes must render the dashboard immediately,
                # with no blank import screen in between.
                driver.get(server.console_url)
                assert driver.title == "KubePreflight Console"
                wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), "?findings= did not auto-load the demo report")
                assert driver.find_element(By.ID, "result-badge").text == "BLOCKED"
                assert driver.find_element(By.ID, "metric-blockers").text == "9"
                assert driver.find_element(By.ID, "metric-warnings").text == "2"
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
                assert len(driver.find_elements(By.CSS_SELECTOR, "table")) == 0, "Findings table should not be mounted on the Summary tab"

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
                assert "BLOCKERS (9)" in actions.text
                assert "WARNINGS (2)" in actions.text
                actions.find_elements(By.CSS_SELECTOR, ".action-copy-button")[0].click()

                # Findings tab: severity chips, confidence, namespace, and
                # text filters, plus the split-pane list + detail.
                click_tab(driver, "Findings")
                wait(driver, lambda d: len(visible_rows(d)) == 11, "findings tab did not render the full table")
                assert driver.find_element(By.ID, "finding-count").text == "11 of 11 findings"

                click_severity_chip(driver, "Blocker")
                wait(driver, lambda d: len(visible_rows(d)) == 2, "severity chip filter failed")
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                Select(driver.find_element(By.ID, "confidence-filter")).select_by_visible_text("STATIC_CERTAIN")
                assert len(visible_rows(driver)) == 11
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                Select(driver.find_element(By.ID, "namespace-filter")).select_by_visible_text("demo")
                assert len(visible_rows(driver)) > 0
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
                wait(driver, lambda d: len(visible_rows(d)) == 11, "clear filters did not restore every chip")

                # Selecting a row shows its detail inline (right pane) —
                # remediation copy, and finding-JSON copy. (The empty state
                # itself — no finding selected yet — is exercised by the
                # Vitest component test; by this point in the flow a
                # finding is already selected from the Top Risks click
                # above, which is itself proof the detail pane responds to
                # a fresh selection too.)
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

                # Uses a synthetic, cluster-independent fixture (fresh
                # findings.json/report.html rendered straight from
                # internal/report, not demo/sample-output) so this guard
                # always reflects the current template/CSS instead of
                # whatever was last captured into the committed fixture.
                with ReportServer(None, synthetic=True) as synth_server:
                    for width, height in widths:
                        driver.set_window_size(width, height)
                        driver.get(synth_server.report_url)
                        assert_no_horizontal_overflow(driver, f"report.html summary @ {width}px")
                        for tab in ("findings", "actions", "evidence"):
                            driver.find_element(By.CSS_SELECTOR, f'.tab-button[data-tab="{tab}"]').click()
                            assert_no_horizontal_overflow(driver, f"report.html {tab} @ {width}px")

                    for width, height in widths:
                        driver.set_window_size(width, height)
                        driver.get(synth_server.console_url)
                        wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), f"console did not auto-load at {width}px")
                        assert_no_horizontal_overflow(driver, f"console summary @ {width}px")

                print("browser smoke: PASS")
            finally:
                driver.quit()


if __name__ == "__main__":
    main()

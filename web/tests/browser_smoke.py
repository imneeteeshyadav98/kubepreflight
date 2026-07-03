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
    URLs."""

    def __init__(self, output_dir, findings_name="findings.json"):
        self.output_dir = output_dir
        self.findings_name = findings_name
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

        self.process = subprocess.Popen(
            [self._binary, "--dir", str(self.output_dir), "--findings", self.findings_name],
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
                assert len(visible_rows(driver)) == 11
                # React unmounts the import panel entirely once a report is
                # loaded (unlike the old vanilla-JS Console, which toggled a
                # `hidden` attribute on an always-present element).
                assert len(driver.find_elements(By.ID, "import-panel")) == 0

                # Severity, confidence, namespace, and text filters.
                Select(driver.find_element(By.ID, "severity-filter")).select_by_visible_text("Warning")
                wait(driver, lambda d: len(visible_rows(d)) == 2, "severity filter failed")
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                Select(driver.find_element(By.ID, "confidence-filter")).select_by_visible_text("STATIC_CERTAIN")
                assert len(visible_rows(driver)) == 11
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                Select(driver.find_element(By.ID, "namespace-filter")).select_by_visible_text("demo")
                assert len(visible_rows(driver)) > 0
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "reset-filters"))
                driver.find_element(By.ID, "search-filter").send_keys("WH-002")
                wait(driver, lambda d: len(visible_rows(d)) == 1, "search filter failed")

                # Detail drawer and remediation copy.
                visible_rows(driver)[0].click()
                wait(driver, lambda d: d.find_element(By.ID, "finding-dialog").get_attribute("open") is not None, "drawer did not open")
                assert driver.find_element(By.ID, "dialog-evidence").text
                driver.find_element(By.ID, "copy-remediation").click()
                wait(driver, lambda d: d.find_element(By.ID, "copy-remediation").text in {"Copied", "Copy unavailable"}, "copy action did not resolve")
                driver.find_element(By.ID, "dialog-close").click()

                # Export produces a local JSON download.
                driver.execute_script("arguments[0].click()", driver.find_element(By.ID, "export-button"))
                wait(driver, lambda _d: any(temp_path.glob("*.json")), "export did not download JSON")

                # Manual "Import findings.json" still overrides whatever was
                # auto-loaded — upload a different, synthetic report and
                # confirm the workspace switches to it.
                warning_fixture = temp_path / "warning.json"
                write_report(warning_fixture, "Warning")
                upload(driver, warning_fixture)
                assert driver.find_element(By.ID, "result-badge").text == "PASSED_WITH_WARNINGS"
                assert driver.find_element(By.ID, "metric-blockers").text == "0"

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

                # Mobile layout keeps the document itself within the viewport
                # (real production auto-load, not a manually triggered demo).
                driver.set_window_size(390, 844)
                driver.get(server.console_url)
                wait(driver, lambda d: d.find_element(By.ID, "workspace").is_displayed(), "mobile auto-load did not render the workspace")
                driver.execute_script("window.scrollTo(1000, 0)")
                assert driver.execute_script("return window.scrollX") == 0, "mobile document scrolls horizontally"
                table_scroll = driver.execute_script("var el=document.querySelector('.table-wrap'); return el.scrollWidth > el.clientWidth")
                assert table_scroll, "wide findings table is not contained in its own scroller"
                driver.save_screenshot(str(temp_path / "console-mobile.png"))

                # The sibling report.html route is unaffected by the Console
                # migration — quick regression check on the same server.
                driver.get(server.report_url)
                assert "KubePreflight Scan Report" in driver.page_source

                print("browser smoke: PASS")
            finally:
                driver.quit()


if __name__ == "__main__":
    main()

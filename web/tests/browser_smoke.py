#!/usr/bin/env python3
"""Functional browser smoke for the local KubePreflight Console.

Run while the repository root is served at http://127.0.0.1:8080.
Requires Selenium and a local Chrome/Chromium installation; no app dependency
or browser driver is checked into the repository.
"""

import json
import os
import tempfile
import time
from pathlib import Path

from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import Select, WebDriverWait


ROOT = Path(__file__).resolve().parents[2]
URL = "http://127.0.0.1:8080/web/"


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

        driver = webdriver.Chrome(options=options)
        try:
            driver.get(URL)
            assert driver.title == "KubePreflight Console"

            # Bundled worst-case demo and derived counts.
            driver.find_element(By.ID, "load-demo-button").click()
            wait(driver, lambda d: d.find_element(By.ID, "metric-blockers").text == "9", "demo blockers did not render")
            assert driver.find_element(By.ID, "result-badge").text == "BLOCKED"
            assert driver.find_element(By.ID, "metric-warnings").text == "2"
            assert len(visible_rows(driver)) == 11

            # Severity, confidence, namespace, and text filters.
            Select(driver.find_element(By.ID, "severity-filter")).select_by_visible_text("Warning")
            wait(driver, lambda d: len(visible_rows(d)) == 2, "severity filter failed")
            driver.find_element(By.ID, "reset-filters").click()
            Select(driver.find_element(By.ID, "confidence-filter")).select_by_visible_text("STATIC_CERTAIN")
            assert len(visible_rows(driver)) == 11
            driver.find_element(By.ID, "reset-filters").click()
            Select(driver.find_element(By.ID, "namespace-filter")).select_by_visible_text("demo")
            assert len(visible_rows(driver)) > 0
            driver.find_element(By.ID, "reset-filters").click()
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

            # Valid file upload works independently of bundled demo fetch.
            driver.get(URL)
            upload(driver, ROOT / "demo/sample-output/findings.json")
            assert driver.find_element(By.ID, "metric-blockers").text == "9"

            # Malformed JSON is rejected safely on the import surface.
            malformed = temp_path / "malformed.json"
            malformed.write_text("{ definitely not json", encoding="utf-8")
            driver.get(URL)
            driver.find_element(By.ID, "file-input").send_keys(str(malformed))
            wait(driver, lambda d: d.find_element(By.ID, "error-message").is_displayed(), "malformed JSON error was not shown")
            assert "Invalid JSON" in driver.find_element(By.ID, "error-message").text

            # CLEAN and warning-only result cards.
            driver.get(URL)
            driver.find_element(By.ID, "load-clean-button").click()
            assert driver.find_element(By.ID, "result-badge").text == "CLEAN"
            warning = temp_path / "warning.json"
            write_report(warning, "Warning")
            driver.get(URL)
            upload(driver, warning)
            assert driver.find_element(By.ID, "result-badge").text == "PASSED_WITH_WARNINGS"

            # Mobile layout keeps the document itself within the viewport; the
            # findings table owns its intentional horizontal scrolling.
            driver.set_window_size(390, 844)
            driver.get(URL)
            driver.find_element(By.ID, "load-clean-button").click()
            driver.execute_script("window.scrollTo(1000, 0)")
            assert driver.execute_script("return window.scrollX") == 0, "mobile document scrolls horizontally"
            table_scroll = driver.execute_script("var el=document.querySelector('.table-wrap'); return el.scrollWidth > el.clientWidth")
            assert table_scroll, "wide findings table is not contained in its own scroller"
            driver.save_screenshot(str(temp_path / "console-mobile.png"))

            print("browser smoke: PASS")
        finally:
            driver.quit()


if __name__ == "__main__":
    main()

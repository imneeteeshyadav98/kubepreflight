# KubePreflight Console

Local, dependency-free viewer for KubePreflight `findings.json` reports. It has
no backend, account system, database, telemetry, or cluster access. Files are
parsed and rendered in the browser.

From the repository root:

```bash
python3 -m http.server 8080
```

Then open <http://localhost:8080/web/>. Serving the repository root (rather than
only `web/`) lets **Load worst-case demo** read
`demo/sample-output/findings.json`.

Features:

- local JSON import and export;
- derived CLEAN / PASSED_WITH_WARNINGS / BLOCKED summary;
- severity, confidence, namespace, and free-text filters;
- structured multi-plane resource display;
- evidence and remediation detail dialog with copy action;
- bundled worst-case and synthetic clean demos;
- responsive layout with no runtime dependencies.

Parser tests use Node's built-in test runner:

```bash
node --test web/tests/*.test.mjs
```

For the functional browser gate, serve the repository as above and run (with
local Chrome/Chromium and Selenium installed):

```bash
python3 web/tests/browser_smoke.py
```

This exercises bundled and uploaded reports, malformed JSON handling, all
result states, filters, the detail drawer, copy/export actions, and mobile
viewport overflow. A human browser pass is still required for visual polish.

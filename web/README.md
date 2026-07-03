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

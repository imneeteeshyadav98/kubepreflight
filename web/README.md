# KubePreflight Console

A React app that turns a KubePreflight `findings.json` into an interactive
decision surface: filters, structured per-finding remediation, grouped next
actions, and an optional multi-hop planner. It has no backend, account system, database, telemetry,
or cluster access — everything is parsed and rendered in the browser.

**End users never install Node.** The Console is built once (`npm run
build`) into static assets under `web/dist/`, which are embedded into the
`kubepreflight` binary via `go:embed` (`web/embed.go`). `kubepreflight scan`
starts a local, local-only HTTP server and serves the embedded Console at
`/console/` alongside the generated `report.html`/`report.md`/
`findings.json` — see the top-level README's "Interactive scans" section.

## Developing

```bash
cd web
npm install
npm run dev       # Vite dev server with hot reload, http://localhost:5173
npm test          # Vitest: schema parser + component tests
npx tsc -b        # type-check only
```

## Building for the Go binary

```bash
cd web
npm run build     # writes web/dist/ (index.html + hashed assets)
```

**`web/dist` must be committed** alongside any `web/src` change — `go:embed`
reads whatever is on disk at Go build time, not at Console-source-edit
time. A stale `web/dist` means `kubepreflight` serves an old Console even
though the source changed, with no error anywhere to catch it. `go build
./...` will fail outright if `web/dist` doesn't exist at all (the
`//go:embed dist` directive requires at least one matching file).

Before committing a `web/src` change, run the freshness guard — it rebuilds
the Console and fails if the result differs from what's committed:

```bash
scripts/check-console-dist.sh
```

This is a release-hygiene script as well as a required CI gate.

## Testing

Component and parser tests (Vitest + Testing Library + jsdom):

```bash
npm test
```

For the functional browser gate — driving the *real* embedded Console
through the *real* local report server (`internal/reportserver`, the exact
code path `kubepreflight scan` uses), not a stand-in static file server —
with local Chrome/Chromium, Selenium, and `go` on PATH:

```bash
python3 web/tests/browser_smoke.py
```

This builds `cmd/consoledevserver` (a dev-only helper, not part of the
public CLI) pointed at a small fixture generated fresh from
`internal/report` (see `writeSyntheticFixture` — no committed demo output
to go stale), then exercises: auto-load via the printed `?findings=` URL,
severity/confidence/namespace/search filters, the finding detail drawer and
remediation copy, JSON export, manual re-import (including malformed-JSON
handling and that a failed re-import doesn't blank out an already-loaded
report), an explicit `?findings=` path that 404s, mobile viewport overflow,
and the sibling `/report.html` route on the same server.

## Architecture

```
web/
  src/
    App.tsx              # top-level state: report, filters, selected finding, errors
    main.tsx              # React root
    lib/findings-schema.ts   # validates + normalizes versioned findings.json
    lib/plan-schema.ts       # validates optional upgrade-plan.json
    lib/actions.ts           # shared resource grouping and global-first ordering
    components/              # dashboard, findings, remediation, actions, planner
  embed.go                 # //go:embed dist — imported by internal/reportserver
  dist/                    # npm run build output (commit this)
```

Auto-load behavior (`App.tsx`, on mount): reads `?findings=` from the URL;
if present, fetches and parses that path and shows a readable error on
failure. If absent, tries the conventional `/findings.json` silently — a
404 there just means no scan has run yet in this server instance, so it
falls back to the ordinary empty/import state rather than showing an error.

// Command consoledevserver starts the real local report server
// (internal/reportserver, the same code path `kubepreflight scan` uses)
// against an existing report output directory, so the Console browser
// smoke test (web/tests/browser_smoke.py) can exercise the actual
// embedded-Console mount at /console/ instead of a stand-in static file
// server. Not part of the public CLI — not built or shipped by the
// Dockerfile, which only compiles ./cmd/kubepreflight.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kubepreflight/internal/reportserver"
)

func main() {
	dir := flag.String("dir", ".", "directory containing report.html and the findings JSON")
	findings := flag.String("findings", "findings.json", "findings file name within dir")
	listen := flag.String("listen", "127.0.0.1:0", "listen address")
	flag.Parse()

	server, err := reportserver.Start(reportserver.Config{Listen: *listen, OutputDir: *dir, FindingsPath: *findings})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("REPORT " + server.ReportURL())
	if consoleURL, ok := server.ConsoleURL(); ok {
		fmt.Println("CONSOLE " + consoleURL)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Wait(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Package reportserver serves generated KubePreflight reports on a local-only
// HTTP listener. It deliberately exposes a small allowlist of report paths,
// never the entire repository or output directory.
package reportserver

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	kpweb "kubepreflight/web"
)

type Config struct {
	Listen       string
	OutputDir    string
	FindingsPath string

	// FallbackOnBusy retries with an OS-assigned free port (same host,
	// port 0) when Listen is already in use. Left false for an explicit
	// user-chosen address, which should fail loudly instead of silently
	// moving to a different port than the one requested.
	FallbackOnBusy bool
	ServePlan      bool
	ServeRollback  bool
}

type Server struct {
	httpServer *http.Server
	listener   net.Listener
	errCh      chan error
	baseURL    string
	hasConsole bool
}

func Start(cfg Config) (*Server, error) {
	if cfg.Listen == "" {
		cfg.Listen = "127.0.0.1:0"
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "."
	}
	root, err := filepath.Abs(cfg.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve report output directory: %w", err)
	}
	findingsPath := cfg.FindingsPath
	if !filepath.IsAbs(findingsPath) {
		findingsPath = filepath.Join(root, findingsPath)
	}

	reportHTML := filepath.Join(root, "report.html")
	if _, err := os.Stat(reportHTML); err != nil {
		return nil, fmt.Errorf("report server requires report.html: %w", err)
	}
	if _, err := os.Stat(findingsPath); err != nil {
		return nil, fmt.Errorf("report server requires findings JSON: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/report.html", http.StatusFound)
	})
	mux.HandleFunc("/report.html", serveExactFile(reportHTML))
	mux.HandleFunc("/findings.json", serveExactFile(findingsPath))
	if _, err := os.Stat(filepath.Join(root, "report.md")); err == nil {
		mux.HandleFunc("/report.md", serveExactFile(filepath.Join(root, "report.md")))
	}
	// Only plan runs opt into this route. A stale upgrade-plan.json left in an
	// output directory after a later scan must never make the Console show an
	// unrelated Planner tab.
	if cfg.ServePlan {
		if _, err := os.Stat(filepath.Join(root, "upgrade-plan.json")); err == nil {
			mux.HandleFunc("/upgrade-plan.json", serveExactFile(filepath.Join(root, "upgrade-plan.json")))
		}
	}
	if cfg.ServeRollback {
		if _, err := os.Stat(filepath.Join(root, "rollback-assessment.json")); err == nil {
			mux.HandleFunc("/rollback-assessment.json", serveExactFile(filepath.Join(root, "rollback-assessment.json")))
		}
	}

	// The Console is a React app built once at release time (web/README.md)
	// and embedded into the binary via go:embed (web/embed.go) — unlike
	// report.html/findings.json, it doesn't live in OutputDir and needs no
	// disk lookup, so every scan can serve it, not just ones run from a
	// checkout that happens to have a web/ directory alongside the output.
	consoleRoot, err := fs.Sub(kpweb.ConsoleFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("open embedded Console assets: %w", err)
	}
	hasConsole := false
	if info, err := fs.Stat(consoleRoot, "index.html"); err == nil && !info.IsDir() {
		hasConsole = true
		mux.Handle("/console/", http.StripPrefix("/console/", http.FileServer(http.FS(consoleRoot))))
	}
	listener, err := net.Listen("tcp", cfg.Listen)
	if err != nil && cfg.FallbackOnBusy && errors.Is(err, syscall.EADDRINUSE) {
		listener, err = net.Listen("tcp", fallbackAddr(cfg.Listen))
	}
	if err != nil {
		return nil, fmt.Errorf("listen for local report server: %w", err)
	}
	s := &Server{
		httpServer: &http.Server{
			Handler:           securityHeaders(mux),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		listener:   listener,
		errCh:      make(chan error, 1),
		baseURL:    "http://" + listener.Addr().String(),
		hasConsole: hasConsole,
	}
	go func() {
		err := s.httpServer.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.errCh <- err
		}
		close(s.errCh)
	}()
	return s, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// fallbackAddr keeps addr's host and swaps its port for 0 (OS-assigned),
// so a fallback still binds loopback-only when the original address did.
// An unparseable addr defensively falls back to the literal
// "127.0.0.1:0" rather than propagating a confusing second error.
func fallbackAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return "127.0.0.1:0"
	}
	return net.JoinHostPort(host, "0")
}

func serveExactFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		http.ServeFile(w, r, path)
	}
}

func (s *Server) ReportURL() string { return s.baseURL + "/report.html" }

func (s *Server) FindingsURL() string { return s.baseURL + "/findings.json" }

// ConsoleURL points at the bundled Console with a ?findings= query param
// pre-filled, so opening the printed link loads the just-completed scan's
// findings automatically instead of landing on the Console's blank import
// screen. The findings route is always the stable /findings.json path
// (see the mux registration above) regardless of the --findings-out
// filename actually used on disk — the server already normalizes that.
func (s *Server) ConsoleURL() (string, bool) {
	if !s.hasConsole {
		return "", false
	}
	return s.baseURL + "/console/?findings=/findings.json#summary", true
}

// Wait blocks until ctx is canceled or the HTTP server fails. Cancellation
// triggers a bounded graceful shutdown so Ctrl+C never strands the listener.
func (s *Server) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down report server: %w", err)
		}
		return nil
	case err := <-s.errCh:
		if err != nil {
			return fmt.Errorf("serve local reports: %w", err)
		}
		return nil
	}
}

// OpenBrowser asks the operating system to open url. Callers intentionally
// treat failures as warnings: report generation and scan results stay valid.
func OpenBrowser(url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{url}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		command, args = "xdg-open", []string{url}
	}
	if err := exec.Command(command, args...).Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}

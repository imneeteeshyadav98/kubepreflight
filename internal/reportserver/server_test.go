package reportserver

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServerRedirectsAndServesAllowedReports(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "report.html"), "<h1>report</h1>")
	writeFixture(t, filepath.Join(dir, "custom.json"), `{"findings":[]}`)
	writeFixture(t, filepath.Join(dir, "private.txt"), "must not be served")

	server, err := Start(Config{Listen: "127.0.0.1:0", OutputDir: dir, FindingsPath: "custom.json"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Wait(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Wait: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("server did not shut down")
		}
	})

	client := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(server.baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") != "/report.html" {
		t.Fatalf("GET / = %d Location=%q, want 302 /report.html", response.StatusCode, response.Header.Get("Location"))
	}

	assertBody(t, server.ReportURL(), "<h1>report</h1>")
	assertBody(t, server.FindingsURL(), `{"findings":[]}`)
	consoleURL, ok := server.ConsoleURL()
	if !ok {
		t.Fatal("ConsoleURL reports no bundled Console")
	}
	// The printed Console link must pre-fill ?findings= so opening it after
	// a scan auto-loads results instead of landing on a blank import
	// screen (the exact bug this test guards: Console used to print a bare
	// /web/ URL with no query param).
	if !strings.Contains(consoleURL, "?findings=/findings.json") {
		t.Fatalf("ConsoleURL = %q, want it to contain ?findings=/findings.json", consoleURL)
	}
	if !strings.HasSuffix(consoleURL, "#summary") {
		t.Fatalf("ConsoleURL = %q, want it to end in #summary", consoleURL)
	}
	// The Console is embedded in the binary (web/embed.go), not read from
	// OutputDir, so this checks the real built web/dist/index.html rather
	// than a fixture — asserting on its stable title instead of an exact
	// byte match, since the bundled asset hashes change on every build.
	response, err = http.Get(consoleURL)
	if err != nil {
		t.Fatalf("GET Console: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read Console body: %v", err)
	}
	if !strings.Contains(string(body), "<title>KubePreflight Console</title>") {
		t.Fatalf("Console body missing expected title, got: %s", body)
	}

	response, err = http.Get(server.baseURL + "/private.txt")
	if err != nil {
		t.Fatalf("GET private path: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("private path status = %d, want 404", response.StatusCode)
	}
}

func reportFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "report.html"), "<h1>report</h1>")
	writeFixture(t, filepath.Join(dir, "findings.json"), `{"findings":[]}`)
	return dir
}

func shutdown(t *testing.T, s *Server) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Wait(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("server did not shut down")
	}
}

// TestStart_ExplicitListenFailsWhenBusy guards the "explicit --listen
// should fail loudly, not silently move elsewhere" requirement:
// FallbackOnBusy defaults to false (its zero value), so a busy address
// must still fail Start outright.
func TestStart_ExplicitListenFailsWhenBusy(t *testing.T) {
	dir := reportFixtureDir(t)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy a port: %v", err)
	}
	defer occupied.Close()

	_, err = Start(Config{Listen: occupied.Addr().String(), OutputDir: dir, FindingsPath: "findings.json"})
	if err == nil {
		t.Fatal("Start succeeded against a busy address, want an error")
	}
}

// TestStart_FallbackOnBusyUsesADifferentPort guards the default-port UX:
// when the caller opts in via FallbackOnBusy, a busy address must not
// fail the whole command — it silently retries on an OS-assigned port
// instead, on the same host.
func TestStart_FallbackOnBusyUsesADifferentPort(t *testing.T) {
	dir := reportFixtureDir(t)

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occupy a port: %v", err)
	}
	defer occupied.Close()
	busyAddr := occupied.Addr().String()

	server, err := Start(Config{Listen: busyAddr, OutputDir: dir, FindingsPath: "findings.json", FallbackOnBusy: true})
	if err != nil {
		t.Fatalf("Start with FallbackOnBusy: %v", err)
	}
	t.Cleanup(func() { shutdown(t, server) })

	want := "http://" + busyAddr + "/report.html"
	if server.ReportURL() == want {
		t.Fatalf("ReportURL() = %q, want a different (fallback) port than the busy address", server.ReportURL())
	}
	assertBody(t, server.ReportURL(), "<h1>report</h1>")
}

func assertBody(t *testing.T, url, want string) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	if string(raw) != want {
		t.Fatalf("GET %s = %q, want %q", url, raw, want)
	}
}

// TestStart_ServesUpgradePlanJSONWhenPresent guards the Console's
// opportunistic-probe design: when a `plan` run's upgrade-plan.json sits
// alongside report.html/findings.json, the server must expose it at a
// stable route so the Console can fetch it without any new CLI flag.
func TestStart_ServesUpgradePlanJSONWhenPresent(t *testing.T) {
	dir := reportFixtureDir(t)
	writeFixture(t, filepath.Join(dir, "upgrade-plan.json"), `{"fromVersion":"1.29","toVersion":"1.30","hops":[]}`)

	server, err := Start(Config{Listen: "127.0.0.1:0", OutputDir: dir, FindingsPath: "findings.json", ServePlan: true})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { shutdown(t, server) })

	assertBody(t, server.baseURL+"/upgrade-plan.json", `{"fromVersion":"1.29","toVersion":"1.30","hops":[]}`)
}

// TestStart_UpgradePlanJSONAbsentDoesNotFailStart guards the "never fails
// Start" requirement (same as report.md's existing optionality): a scan
// (not plan) run has no upgrade-plan.json at all, and the server must
// still start cleanly and 404 that path rather than erroring.
func TestStart_UpgradePlanJSONAbsentDoesNotFailStart(t *testing.T) {
	dir := reportFixtureDir(t)
	writeFixture(t, filepath.Join(dir, "upgrade-plan.json"), `{"stale":true}`)

	server, err := Start(Config{Listen: "127.0.0.1:0", OutputDir: dir, FindingsPath: "findings.json"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { shutdown(t, server) })

	response, err := http.Get(server.baseURL + "/upgrade-plan.json")
	if err != nil {
		t.Fatalf("GET /upgrade-plan.json: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /upgrade-plan.json status = %d, want 404 when plan serving is disabled even if a stale file exists", response.StatusCode)
	}
}

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

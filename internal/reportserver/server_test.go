package reportserver

import (
	"context"
	"io"
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

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

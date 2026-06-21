package dashboard

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeAnalysisJSON(t *testing.T, dir string) string {
	t.Helper()
	forgeDir := filepath.Join(dir, ".forge")
	if err := os.MkdirAll(forgeDir, 0o755); err != nil {
		t.Fatalf("mkdir .forge: %v", err)
	}
	analysis := map[string]any{
		"version": "1",
		"project": map[string]any{
			"name":       "testproject",
			"frameworks": []string{"nextjs"},
		},
		"files": []map[string]any{
			{"id": "f1", "path": "main.go"},
		},
	}
	data, _ := json.Marshal(analysis)
	path := filepath.Join(forgeDir, "analysis.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write analysis.json: %v", err)
	}
	return path
}

func TestFindAvailablePort(t *testing.T) {
	port, ln, err := findAvailablePort()
	if err != nil {
		t.Fatalf("findAvailablePort() error: %v", err)
	}
	defer ln.Close()

	if port < portRangeStart || port >= portRangeEnd {
		t.Errorf("port %d is outside expected range [%d, %d)", port, portRangeStart, portRangeEnd)
	}
}

func TestNewServerCreatesValidServer(t *testing.T) {
	dir := t.TempDir()
	analysisPath := writeAnalysisJSON(t, dir)

	srv, err := NewServer(analysisPath)
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}
	defer srv.listener.Close()

	if srv.port < portRangeStart || srv.port >= portRangeEnd {
		t.Errorf("server port %d out of range", srv.port)
	}
	if srv.analysisPath != analysisPath {
		t.Errorf("analysisPath = %q, want %q", srv.analysisPath, analysisPath)
	}
}

func TestNewServerRejectsNonexistentAnalysis(t *testing.T) {
	_, err := NewServer("/nonexistent/analysis.json")
	if err == nil {
		t.Fatal("NewServer() with nonexistent path: want error, got nil")
	}
}

// startTestServer creates a server and starts it in the background.
// The server stops when the returned cancel is called.
func startTestServer(t *testing.T, analysisPath string) (*Server, context.CancelFunc) {
	t.Helper()
	srv, err := NewServer(analysisPath)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go srv.Start(ctx) //nolint:errcheck
	// Give the goroutine a moment to begin serving.
	time.Sleep(10 * time.Millisecond)
	return srv, cancel
}

func TestServerServesIndexHTML(t *testing.T) {
	dir := t.TempDir()
	analysisPath := writeAnalysisJSON(t, dir)
	srv, cancel := startTestServer(t, analysisPath)
	defer cancel()

	resp, err := http.Get(srv.URL() + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !containsString(body, "forge dashboard") {
		t.Errorf("GET / body does not contain 'forge dashboard'; got:\n%s", body)
	}
}

func TestServerServesAnalysis(t *testing.T) {
	dir := t.TempDir()
	analysisPath := writeAnalysisJSON(t, dir)
	srv, cancel := startTestServer(t, analysisPath)
	defer cancel()

	resp, err := http.Get(srv.URL() + "/api/analysis")
	if err != nil {
		t.Fatalf("GET /api/analysis: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/analysis status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, body)
	}
	proj, _ := got["project"].(map[string]any)
	if proj == nil || proj["name"] != "testproject" {
		t.Errorf("project.name: want testproject, got %v", proj)
	}
}

func TestServerHealthEndpoint(t *testing.T) {
	dir := t.TempDir()
	analysisPath := writeAnalysisJSON(t, dir)
	srv, cancel := startTestServer(t, analysisPath)
	defer cancel()

	resp, err := http.Get(srv.URL() + "/api/health")
	if err != nil {
		t.Fatalf("GET /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/health status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var health map[string]string
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatalf("health response not valid JSON: %v", err)
	}
	if health["status"] != "ok" {
		t.Errorf("health status = %q, want ok", health["status"])
	}
}

func TestServerShutdownOnContext(t *testing.T) {
	dir := t.TempDir()
	analysisPath := writeAnalysisJSON(t, dir)
	srv, err := NewServer(analysisPath)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// clean shutdown
	case <-time.After(2 * time.Second):
		t.Error("server did not shut down within 2s of context cancellation")
	}
}

func containsString(body []byte, s string) bool {
	return len(body) > 0 && indexBytes(body, []byte(s)) >= 0
}

func indexBytes(hay, needle []byte) int {
	for i := 0; i <= len(hay)-len(needle); i++ {
		if string(hay[i:i+len(needle)]) == string(needle) {
			return i
		}
	}
	return -1
}

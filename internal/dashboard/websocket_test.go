package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHubRegistersClient(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(20 * time.Millisecond)
	if got := h.ClientCount(); got != 1 {
		t.Errorf("ClientCount = %d, want 1", got)
	}
}

func TestHubBroadcasts(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"

	conn1, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial conn1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial conn2: %v", err)
	}
	defer conn2.Close()

	time.Sleep(20 * time.Millisecond)

	h.BroadcastAnalysisUpdate()

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)) //nolint:errcheck
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("conn%d: ReadMessage: %v", i+1, err)
			continue
		}
		if !strings.Contains(string(msg), "analysis_updated") {
			t.Errorf("conn%d: unexpected message: %s", i+1, msg)
		}
	}
}

func TestHubTracksLastEmpty(t *testing.T) {
	h := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(h.ServeWS))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if got := h.IdleSeconds(); got != 0 {
		t.Errorf("IdleSeconds while connected = %d, want 0", got)
	}

	conn.Close()
	// Wait long enough that idleDuration() rounds to at least 1 whole second.
	time.Sleep(1100 * time.Millisecond)

	if got := h.IdleSeconds(); got == 0 {
		t.Error("IdleSeconds after disconnect = 0, want >= 1")
	}
}

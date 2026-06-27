package dashboard

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub manages active WebSocket connections and broadcasts updates.
type Hub struct {
	clients   map[*wsClient]bool
	mu        sync.RWMutex
	lastEmpty time.Time
}

// wsClient represents a single WebSocket connection.
type wsClient struct {
	hub  *Hub
	conn *websocket.Conn
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewHub creates an empty Hub. The idle clock starts immediately so
// servers that never receive a browser connection will still time out.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*wsClient]bool),
		lastEmpty: time.Now(),
	}
}

// Register adds a client. Resets the idle clock (clients are active).
func (h *Hub) Register(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = true
	h.lastEmpty = time.Time{}
	h.mu.Unlock()
}

// Unregister removes a client. Records lastEmpty if no clients remain.
func (h *Hub) Unregister(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	if len(h.clients) == 0 {
		h.lastEmpty = time.Now()
	}
	h.mu.Unlock()
}

// BroadcastAnalysisUpdate sends {"type":"analysis_updated"} to all clients.
func (h *Hub) BroadcastAnalysisUpdate() {
	msg, _ := json.Marshal(map[string]string{"type": "analysis_updated"})
	h.mu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("dashboard: ws send: %v", err)
			h.Unregister(c)
			c.conn.Close()
		}
	}
}

// IdleSeconds returns whole seconds since the last client disconnected (or
// since hub creation if no client has ever connected). Returns 0 if any
// client is currently connected.
func (h *Hub) IdleSeconds() int {
	return int(h.idleDuration().Seconds())
}

// idleDuration returns the precise duration since the hub became idle.
// Returns 0 if any client is connected.
func (h *Hub) idleDuration() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) > 0 {
		return 0
	}
	return time.Since(h.lastEmpty)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ServeWS upgrades an HTTP connection to WebSocket and registers the client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("dashboard: ws upgrade: %v", err)
		return
	}
	c := &wsClient{hub: h, conn: conn}
	h.Register(c)
	go c.readPump()
}

// readPump drains incoming messages and detects client disconnect.
func (c *wsClient) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

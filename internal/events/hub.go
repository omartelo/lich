// Package events pushes app events (session status, titles, touched —
// anything the backend pushes to the UI) to the local WebSocket
// client on /events (see docs/chromium-shell.md). While no client is
// connected events are dropped — the window owns the connection, so nobody
// is listening anyway. It is also the delivery channel terminal I/O falls
// back to when the dedicated /ws transport is down.
package events

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// writeTimeout bounds a send to the local client; a stalled write drops the
// connection instead of blocking every subsequent emit.
const writeTimeout = 5 * time.Second

// Envelope is one pushed event as it crosses the /events socket.
type Envelope struct {
	Name string `json:"name"`
	Data any    `json:"data,omitempty"`
}

type Hub struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func New() *Hub {
	return &Hub{}
}

// Emit pushes one event to the connected client; without one it is dropped.
func (h *Hub) Emit(name string, data any) {
	h.mu.Lock()
	conn := h.conn
	h.mu.Unlock()
	if conn == nil {
		return
	}
	payload, err := json.Marshal(Envelope{Name: name, Data: data})
	if err != nil {
		log.Printf("events: marshal %s: %v", name, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		h.drop(conn)
	}
}

func (h *Hub) attach(conn *websocket.Conn) {
	h.mu.Lock()
	previous := h.conn
	h.conn = conn
	h.mu.Unlock()
	if previous != nil {
		_ = previous.CloseNow()
	}
}

func (h *Hub) drop(conn *websocket.Conn) {
	h.mu.Lock()
	if h.conn == conn {
		h.conn = nil
	}
	h.mu.Unlock()
	_ = conn.CloseNow()
}

// ServeHTTP upgrades /events. One client is expected (the shell); a new
// connection replaces the previous one. The read loop only watches for close.
// Token auth is applied by the transport mount, like every mounted handler.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	})
	if err != nil {
		log.Printf("events: accept: %v", err)
		return
	}
	h.attach(conn)
	ctx := context.Background()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			h.drop(conn)
			return
		}
	}
}

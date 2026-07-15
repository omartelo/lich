// Package events routes app events (session status, attention, titles,
// touched — anything the backend pushes to the UI) to exactly one channel: a
// local WebSocket client on /events when one is connected (the Chromium shell
// path of docs/chromium-shell.md), or a fallback emitter (the Wails event
// bridge) otherwise. Routing backend-side keeps the frontend free to listen
// on both channels without double delivery — the same contract the terminal
// data transport already follows.
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
// connection so events fall back to the fallback emitter.
const writeTimeout = 5 * time.Second

// Envelope is one pushed event as it crosses the /events socket.
type Envelope struct {
	Name string `json:"name"`
	Data any    `json:"data,omitempty"`
}

type Hub struct {
	mu       sync.Mutex
	conn     *websocket.Conn
	fallback func(name string, data any)
}

// New returns a hub that falls back to emit (may be nil — events are then
// dropped while no client is connected, acceptable only in tests).
func New(fallback func(name string, data any)) *Hub {
	return &Hub{fallback: fallback}
}

// Emit pushes one event through the connected client or the fallback.
func (h *Hub) Emit(name string, data any) {
	h.mu.Lock()
	conn := h.conn
	h.mu.Unlock()
	if conn == nil {
		if h.fallback != nil {
			h.fallback(name, data)
		}
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
		if h.fallback != nil {
			h.fallback(name, data)
		}
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

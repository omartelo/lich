package main

import (
	"encoding/json"
	"fmt"
)

// Control messages ride text frames, client → server only; PTY output rides
// binary frames server → client. Kept deliberately simpler than the app's
// multiplexed transport (internal/terminal/transport.go): the spike serves a
// single session per connection.
type controlMsg struct {
	T    string `json:"t"` // "in" (keyboard data) | "rs" (resize)
	D    string `json:"d,omitempty"`
	Cols uint16 `json:"c,omitempty"`
	Rows uint16 `json:"r,omitempty"`
}

func parseControl(data []byte) (controlMsg, error) {
	var msg controlMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		return controlMsg{}, fmt.Errorf("bad control frame: %w", err)
	}
	switch msg.T {
	case "in", "rs":
		return msg, nil
	default:
		return controlMsg{}, fmt.Errorf("unknown control type %q", msg.T)
	}
}

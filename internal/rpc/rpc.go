// Package rpc exposes the app's Go services to the frontend over plain HTTP
// on the local transport listener — the Chromium --app window has no binding
// bridge of its own (docs/chromium-shell.md). One POST per call:
//
//	POST /rpc/<service>.<Method>?token=...   body: JSON array of arguments
//	→ 200 with the method's JSON result (null when it only returns error)
//	→ 4xx/5xx with {"error": "..."} otherwise
//
// Dispatch is reflection over explicitly registered services — the net/rpc
// pattern — so a new service method is exposed by registration alone. Token
// auth is applied by the transport mount, not here.
package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
)

// bodyLimit bounds one RPC request body; arguments are ids, paths and small
// strings, so anything larger is malformed or hostile.
const bodyLimit = 1 << 20

var errType = reflect.TypeFor[error]()

type Handler struct {
	services map[string]reflect.Value
	denied   map[string]bool
}

func New() *Handler {
	return &Handler{
		services: make(map[string]reflect.Value),
		denied:   make(map[string]bool),
	}
}

// Register exposes every exported method of svc under name.<Method>.
func (h *Handler) Register(name string, svc any) {
	h.services[name] = reflect.ValueOf(svc)
}

// Deny hides one method ("store.Close") from dispatch — for methods that are
// public for Go callers but must not be reachable from the frontend.
func (h *Handler) Deny(method string) {
	h.denied[method] = true
}

// ServeHTTP handles one call. CORS is wide open on purpose: in dev the page's
// origin is the Vite server, not this listener, and the random token —
// enforced by the transport mount — is the actual auth, as with /ws.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/rpc/")
	method, err := h.lookup(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, bodyLimit))
	if err != nil {
		writeError(w, http.StatusBadRequest, "unreadable body")
		return
	}
	args, err := decodeArgs(method.Type(), body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeResult(w, method.Call(args))
}

func (h *Handler) lookup(name string) (reflect.Value, error) {
	if h.denied[name] {
		return reflect.Value{}, fmt.Errorf("method %q not available", name)
	}
	service, methodName, ok := strings.Cut(name, ".")
	if !ok {
		return reflect.Value{}, fmt.Errorf("malformed method %q", name)
	}
	svc, ok := h.services[service]
	if !ok {
		return reflect.Value{}, fmt.Errorf("unknown service %q", service)
	}
	method := svc.MethodByName(methodName)
	if !method.IsValid() {
		return reflect.Value{}, fmt.Errorf("unknown method %q", name)
	}
	return method, nil
}

// decodeArgs unmarshals the JSON argument array positionally into the
// method's parameter types.
func decodeArgs(t reflect.Type, body []byte) ([]reflect.Value, error) {
	var raw []json.RawMessage
	if len(body) > 0 {
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("arguments must be a JSON array: %w", err)
		}
	}
	if len(raw) != t.NumIn() {
		return nil, fmt.Errorf("want %d arguments, got %d", t.NumIn(), len(raw))
	}
	args := make([]reflect.Value, t.NumIn())
	for i := range args {
		value := reflect.New(t.In(i))
		if err := json.Unmarshal(raw[i], value.Interface()); err != nil {
			return nil, fmt.Errorf("argument %d: %w", i, err)
		}
		args[i] = value.Elem()
	}
	return args, nil
}

// writeResult maps Go returns onto the response: a trailing non-nil error is
// a 500, otherwise the first non-error value (or null) is the 200 body.
func writeResult(w http.ResponseWriter, results []reflect.Value) {
	var payload any
	for _, result := range results {
		if result.Type().Implements(errType) {
			if err, _ := result.Interface().(error); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			continue
		}
		if payload == nil {
			payload = result.Interface()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// Headers are gone; nothing to do but log-free best effort.
		_ = err
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

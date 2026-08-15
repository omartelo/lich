package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/drop"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/rpc"
	"github.com/omartelo/lich/internal/store"
)

// denied spells out denyInternal's list a second time, and names the type each
// entry belongs to. Reading the list under test would prove only that Go can
// range over a slice; written out here, an entry dropped from denyInternal
// fails TestDeniedMethodsAreUnreachable and an entry that names nothing —
// Deny takes any string — fails TestDeniedMethodsExist.
var denied = map[string]reflect.Type{
	"store.Close":         reflect.TypeFor[*store.Service](),
	"drop.Upload":         reflect.TypeFor[*drop.Service](),
	"drop.Save":           reflect.TypeFor[*drop.Service](),
	"relay.Observe":       reflect.TypeFor[*relay.Service](),
	"relay.SetPlugins":    reflect.TypeFor[*relay.Service](),
	"project.SetAccounts": reflect.TypeFor[*project.Service](),
	"project.SetProjects": reflect.TypeFor[*project.Service](),
}

// stubService stands in for the four registered services: dispatch resolves a
// method by name alone, and the real ones want a database, a config directory
// and a running terminal.
type stubService struct{}

func (stubService) Close() error       { return nil }
func (stubService) Upload() error      { return nil }
func (stubService) Save() error        { return nil }
func (stubService) Observe() error     { return nil }
func (stubService) SetPlugins() error  { return nil }
func (stubService) SetAccounts() error { return nil }
func (stubService) SetProjects() error { return nil }
func (stubService) Allowed() error     { return nil }

func denyingDispatcher() *rpc.Handler {
	d := rpc.New()
	for _, service := range []string{"store", "drop", "relay", "project"} {
		d.Register(service, stubService{})
	}
	denyInternal(d)
	return d
}

func post(t *testing.T, d *rpc.Handler, method string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc/"+method, strings.NewReader("[]"))
	rec := httptest.NewRecorder()
	d.ServeHTTP(rec, req)
	return rec
}

func TestDeniedMethodsAreUnreachable(t *testing.T) {
	d := denyingDispatcher()
	for method := range denied {
		if rec := post(t, d, method); rec.Code != http.StatusNotFound {
			t.Errorf("POST /rpc/%s: want 404, got %d", method, rec.Code)
		}
	}
	// The control: the same registration answers everything else, so the 404s
	// above are the deny list and not a service that was never registered.
	if rec := post(t, d, "relay.Allowed"); rec.Code != http.StatusOK {
		t.Errorf("POST /rpc/relay.Allowed: want 200, got %d", rec.Code)
	}
}

func TestDeniedMethodsExist(t *testing.T) {
	for name, service := range denied {
		_, method, _ := strings.Cut(name, ".")
		if _, ok := service.MethodByName(method); !ok {
			t.Errorf("%s: %s has no method %s — the Deny entry hides nothing", name, service, method)
		}
	}
}

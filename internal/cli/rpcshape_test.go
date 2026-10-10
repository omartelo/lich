package cli

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/spawn"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/system"
)

// rpcMethodName is how the CLI spells a method it posts: "<service>.<Method>".
var rpcMethodName = regexp.MustCompile(`^[a-z]+\.[A-Z][A-Za-z]*$`)

// cliServices are the services the CLI posts to, by the name main.go registers
// them under.
var cliServices = map[string]reflect.Type{
	"spawn":  reflect.TypeFor[*spawn.Service](),
	"relay":  reflect.TypeFor[*relay.Service](),
	"store":  reflect.TypeFor[*store.Service](),
	"system": reflect.TypeFor[*system.Service](),
}

// TestEveryCallTheCLIPostsTakesOneOptionsObject keeps a call `lich` or `lich
// mcp` makes from going back to positional arguments. The RPC matches those by
// position and count, and a package manager replaces the binary under a running
// backend, so a positional call breaks on the day its argument list changes
// (docs/ceilings.md). One JSON object, after an optional context, is what lets
// a newer and an older lich understand each other. A call with no input yet,
// like the version probe, has nothing to misalign and may take nothing; the day
// it takes something, that something is the object.
func TestEveryCallTheCLIPostsTakesOneOptionsObject(t *testing.T) {
	methods := postedMethods(t)
	if !methods["spawn.Open"] {
		t.Fatalf("found %v, want spawn.Open among them: the scan no longer sees the CLI's calls", methods)
	}
	for name := range methods {
		service, method, _ := strings.Cut(name, ".")
		svc, ok := cliServices[service]
		if !ok {
			t.Errorf("%s: the CLI posts to service %q, which this test does not know; add it to cliServices", name, service)
			continue
		}
		m, ok := svc.MethodByName(method)
		if !ok {
			t.Errorf("%s: no such method on %s", name, svc)
			continue
		}
		if !takesOneObjectOrNothing(m.Type) {
			t.Errorf("%s takes %s; a call the CLI posts takes one options struct, after an optional context.Context",
				name, m.Type)
		}
	}
}

// postedMethods reads the method names out of the package's own source: every
// string literal shaped like one, which is how each call and each method
// constant spells it.
func postedMethods(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("list the package: %v", err)
	}
	fset := token.NewFileSet()
	found := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if value, err := strconv.Unquote(lit.Value); err == nil && rpcMethodName.MatchString(value) {
				found[value] = true
			}
			return true
		})
	}
	return found
}

// takesOneObjectOrNothing is whether a method's parameters, receiver and an
// optional leading context.Context aside, are one struct or none.
func takesOneObjectOrNothing(method reflect.Type) bool {
	params := make([]reflect.Type, 0, method.NumIn())
	for i := 1; i < method.NumIn(); i++ {
		params = append(params, method.In(i))
	}
	if len(params) > 0 && params[0] == reflect.TypeFor[context.Context]() {
		params = params[1:]
	}
	return len(params) == 0 || len(params) == 1 && params[0].Kind() == reflect.Struct
}

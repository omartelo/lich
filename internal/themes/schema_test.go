package themes

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var updateSchema = flag.Bool("update-schema", false, "rewrite "+SchemaPath+" from Schema()")

const regenerate = "go test ./internal/themes -run TestSchemaFileMatchesGenerator -update-schema"

// TestSchemaFileMatchesGenerator is the whole reason the schema is generated:
// without it a token added to the bundled theme leaves the published schema
// rejecting valid files, which is worse than shipping no schema at all.
func TestSchemaFileMatchesGenerator(t *testing.T) {
	want, err := Schema()
	if err != nil {
		t.Fatalf("Schema() error: %v", err)
	}
	path := filepath.Join("..", "..", filepath.FromSlash(SchemaPath))
	if *updateSchema {
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatalf("write %s: %v", SchemaPath, err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (regenerate with: %s)", SchemaPath, err, regenerate)
	}
	// The repository carries no .gitattributes, so a Windows checkout may hand
	// back CRLF for a file committed with LF. That is the checkout's spelling,
	// not drift.
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		t.Fatalf("%s is stale; regenerate with: %s", SchemaPath, regenerate)
	}
}

// TestSchemaCoversEveryToken pins what the file is worth rather than that it is
// current: a generator that stopped reading appTokens would still round-trip
// through the drift test above once somebody regenerated it.
func TestSchemaCoversEveryToken(t *testing.T) {
	data, err := Schema()
	if err != nil {
		t.Fatalf("Schema() error: %v", err)
	}
	var schema struct {
		Properties map[string]colorGroupSchema `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode generated schema: %v", err)
	}
	assertTokens(t, "app", schema.Properties["app"], appTokens, sortedKeys(appTokens))
	assertTokens(t, "terminal", schema.Properties["terminal"], terminalTokens, []string{"background", "foreground"})
}

type colorGroupSchema struct {
	AdditionalProperties bool                `json:"additionalProperties"`
	Required             []string            `json:"required"`
	Properties           map[string]struct{} `json:"properties"`
}

func assertTokens(t *testing.T, group string, got colorGroupSchema, tokens map[string]struct{}, required []string) {
	t.Helper()
	if got.AdditionalProperties {
		t.Errorf("%s: schema accepts unknown tokens, validateColors rejects them", group)
	}
	if !slices.Equal(sortedKeys(got.Properties), sortedKeys(tokens)) {
		t.Errorf("%s: schema names %v, want %v", group, sortedKeys(got.Properties), sortedKeys(tokens))
	}
	if !slices.Equal(got.Required, required) {
		t.Errorf("%s: schema requires %v, want %v", group, got.Required, required)
	}
}

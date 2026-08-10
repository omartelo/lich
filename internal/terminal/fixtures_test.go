package terminal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The hook contracts in docs/hooks/ are implemented in two repositories: lich
// owns the endpoints, the companion plugin owns the scripts that POST to them.
// docs/hooks/fixtures/ is those contracts as bytes, and these tests are lich's
// half of the deal — the plugin asserts against the same lines, so a payload
// that moves on one side goes red on the other instead of silently drifting.
// The format is documented in docs/hooks/fixtures/README.md.

// hookFixtureDir holds one .jsonl per contract, relative to this package.
const hookFixtureDir = "../../docs/hooks/fixtures"

// hookFixtureCase is one line of a fixture file. Body and Raw are alternatives:
// Raw carries the bodies that are not valid JSON and so cannot be embedded as
// one. Accept and Reject are alternatives too — Accept lists the field values
// parsing must produce, Reject says the payload is refused (its prose is
// documentation, since lich's error strings are its own).
type hookFixtureCase struct {
	Name   string          `json:"name"`
	Body   json.RawMessage `json:"body"`
	Raw    *string         `json:"raw"`
	Accept map[string]any  `json:"accept"`
	Reject string          `json:"reject"`
}

// payload is the exact request body a client sends for this case.
func (c hookFixtureCase) payload() []byte {
	if c.Raw != nil {
		return []byte(*c.Raw)
	}
	return c.Body
}

// hookContract binds a fixture file to the endpoint and the parser it pins. Its
// file is named after the contract document, not the endpoint: /hook predates
// the naming and keeps its path for the plugin releases pointing at it.
type hookContract struct {
	file  string
	path  string
	parse func([]byte) (any, error)
}

// hookContracts is the full set. A contract missing from here, or a fixture
// file missing from the directory, fails TestHookFixturesCoverEveryContract —
// that is what stops a new hook from shipping without its fixtures.
var hookContracts = []hookContract{
	{"session-state.jsonl", "/hook", func(b []byte) (any, error) {
		return parseHookRequest(b)
	}},
	{"session-start.jsonl", "/session-start", func(b []byte) (any, error) {
		return parseSessionStart(b)
	}},
	{"session-title.jsonl", "/session-title", func(b []byte) (any, error) {
		return parseSessionTitle(b)
	}},
	{"session-touched.jsonl", "/session-touched", func(b []byte) (any, error) {
		return parseSessionTouched(b)
	}},
}

// loadHookFixtures reads one fixture file, skipping blank lines.
func loadHookFixtures(t *testing.T, file string) []hookFixtureCase {
	t.Helper()
	f, err := os.Open(filepath.Join(hookFixtureDir, file))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	var cases []hookFixtureCase
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var c hookFixtureCase
		if err := json.Unmarshal([]byte(text), &c); err != nil {
			t.Fatalf("%s:%d is not a JSON object: %v", file, line, err)
		}
		cases = append(cases, c)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read fixture %s: %v", file, err)
	}
	return cases
}

// TestHookFixturesCoverEveryContract pairs the directory with the table above,
// in both directions: a contract without fixtures and a fixture file without a
// contract are both failures.
func TestHookFixturesCoverEveryContract(t *testing.T) {
	found, err := filepath.Glob(filepath.Join(hookFixtureDir, "*.jsonl"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	onDisk := make(map[string]bool, len(found))
	for _, path := range found {
		onDisk[filepath.Base(path)] = true
	}
	for _, contract := range hookContracts {
		if !onDisk[contract.file] {
			t.Errorf("contract %s has no fixture file", contract.path)
		}
		delete(onDisk, contract.file)
	}
	for file := range onDisk {
		t.Errorf("fixture %s pins no contract in hookContracts", file)
	}
}

// TestHookFixturesAreWellFormed holds the format documented in the fixtures
// README, so a malformed case fails as itself rather than as a confusing
// failure in one of the tests below.
func TestHookFixturesAreWellFormed(t *testing.T) {
	for _, contract := range hookContracts {
		t.Run(contract.file, func(t *testing.T) {
			cases := loadHookFixtures(t, contract.file)
			seen := make(map[string]bool, len(cases))
			accepts, rejects := 0, 0
			for i, c := range cases {
				checkFixtureShape(t, i, c)
				if seen[c.Name] {
					t.Errorf("case %d: duplicate name %q", i, c.Name)
				}
				seen[c.Name] = true
				if c.Accept != nil {
					accepts++
				} else {
					rejects++
				}
			}
			if accepts == 0 || rejects == 0 {
				t.Errorf("got %d accept and %d reject cases, want at least one of each",
					accepts, rejects)
			}
		})
	}
}

// checkFixtureShape holds one case to the exactly-one-of rules.
func checkFixtureShape(t *testing.T, i int, c hookFixtureCase) {
	t.Helper()
	if c.Name == "" {
		t.Errorf("case %d: missing name", i)
	}
	if (c.Body == nil) == (c.Raw == nil) {
		t.Errorf("case %q: want exactly one of body / raw", c.Name)
	}
	if (c.Accept == nil) == (c.Reject == "") {
		t.Errorf("case %q: want exactly one of accept / reject", c.Name)
	}
}

// TestHookFixturesMatchParsers runs every case through its contract's parser:
// a reject case must error, and an accept case must produce exactly the field
// values the fixture names, after the contract's defaulting and trimming.
func TestHookFixturesMatchParsers(t *testing.T) {
	for _, contract := range hookContracts {
		t.Run(contract.file, func(t *testing.T) {
			for _, c := range loadHookFixtures(t, contract.file) {
				t.Run(c.Name, func(t *testing.T) {
					parsed, err := contract.parse(c.payload())
					if c.Accept == nil {
						if err == nil {
							t.Fatalf("parsed a payload the contract rejects (%s)", c.Reject)
						}
						return
					}
					if err != nil {
						t.Fatalf("rejected a payload the contract accepts: %v", err)
					}
					assertFixtureFields(t, parsed, c.Accept)
				})
			}
		})
	}
}

// assertFixtureFields compares a parsed request against a case's accept map,
// through the JSON tags the contract is written in. Only the named fields are
// asserted: a field the case is not about stays out of its expectations.
func assertFixtureFields(t *testing.T, parsed any, want map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal parsed request: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode parsed request: %v", err)
	}
	for field, wantValue := range want {
		gotValue, ok := got[field]
		if !ok {
			t.Errorf("accept names %q, which the request has no field for", field)
			continue
		}
		if !reflect.DeepEqual(gotValue, wantValue) {
			t.Errorf("%s = %#v, want %#v", field, gotValue, wantValue)
		}
	}
}

// TestHookFixturesMatchEndpoints POSTs every case to a live transport, so the
// fixtures pin the wire contract a hook script actually meets — auth, body
// limit, parse and apply — and not just the parser underneath it.
func TestHookFixturesMatchEndpoints(t *testing.T) {
	tr, err := newTransport(
		func(string, []byte) {},
		func(hookRequest) {},
		func(string, string, string) error { return nil },
		func(string, string) error { return nil },
		func(string) {},
	)
	if err != nil {
		t.Fatalf("newTransport: %v", err)
	}
	for _, contract := range hookContracts {
		t.Run(contract.file, func(t *testing.T) {
			url := fmt.Sprintf("http://127.0.0.1:%d%s?token=%s", tr.port, contract.path, tr.token)
			for _, c := range loadHookFixtures(t, contract.file) {
				t.Run(c.Name, func(t *testing.T) {
					want := http.StatusBadRequest
					if c.Accept != nil {
						want = http.StatusNoContent
					}
					assertPostStatus(t, url, c.payload(), want)
				})
			}
		})
	}
}

// assertPostStatus posts one fixture body and checks the status code.
func assertPostStatus(t *testing.T, url string, body []byte, want int) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

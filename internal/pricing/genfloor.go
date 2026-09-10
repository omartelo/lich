//go:build ignore

// Command genfloor rewrites prices.json from LiteLLM's table — the same source
// the runtime refresh reads, so the baked floor and a refreshed install price a
// model the same way. Run it when cutting a release:
//
//	task pricing:refresh
//
// Which models the floor carries stays a hand-curated decision: this only
// reprices the ids already in the file, and reports one that the table has
// stopped publishing rather than dropping it. The field names below mirror
// remoteEntry in pricing.go; that file is the one that decides what a price is.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"
)

const (
	remoteURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	floorPath = "internal/pricing/prices.json"
	bodyLimit = 16 << 20
)

// remote is LiteLLM's shape; rate is lich's. omitempty keeps an unpublished
// price absent rather than writing a zero, which is what the runtime reads as
// "this model has no price for that" (see Rate.Cost).
type remote struct {
	Input        float64 `json:"input_cost_per_token"`
	Output       float64 `json:"output_cost_per_token"`
	CacheRead    float64 `json:"cache_read_input_token_cost"`
	CacheWrite   float64 `json:"cache_creation_input_token_cost"`
	CacheWrite1h float64 `json:"cache_creation_input_token_cost_above_1hr"`
}

type rate struct {
	Input        float64 `json:"input,omitempty"`
	Output       float64 `json:"output,omitempty"`
	CacheRead    float64 `json:"cacheRead,omitempty"`
	CacheWrite   float64 `json:"cacheWrite,omitempty"`
	CacheWrite1h float64 `json:"cacheWrite1h,omitempty"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genfloor:", err)
		os.Exit(1)
	}
}

func run() error {
	floor := map[string]rate{}
	current, err := os.ReadFile(floorPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(current, &floor); err != nil {
		return err
	}

	table, err := fetch()
	if err != nil {
		return err
	}

	stale := []string{}
	for model := range floor {
		entry, ok := table[model]
		if !ok {
			stale = append(stale, model)
			continue
		}
		if entry.Input == 0 && entry.Output == 0 {
			stale = append(stale, model)
			continue
		}
		floor[model] = rate(entry)
	}
	sort.Strings(stale)
	for _, model := range stale {
		fmt.Fprintf(os.Stderr, "genfloor: %s is no longer priced upstream — kept at its baked rate\n", model)
	}

	out, err := json.MarshalIndent(floor, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(floorPath, append(out, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("genfloor: repriced %d of %d models\n", len(floor)-len(stale), len(floor))
	return nil
}

func fetch() (map[string]remote, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(remoteURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", remoteURL, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return nil, err
	}
	var table map[string]remote
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, err
	}
	return table, nil
}

// Package pricing answers what a model's tokens cost, in US dollars, for the
// per-session cost readout. It is keyed by model id and knows nothing about who
// ran the model: a second provider's reader gets its prices from the same table.
//
// The table has two layers. A small baked one (prices.json, the Claude models
// known at build time) is the floor, so a fresh install prices correctly with no
// network at all. A model the floor has never heard of — one released after this
// build — triggers a single remote refresh, whose result is cached under the
// config dir and overlays the floor from then on. Nothing here blocks a caller:
// the refresh runs in the background and the lookup that missed simply reports
// "unknown", which the readout renders as absent rather than as zero. Pricing a
// model at nothing is the one answer this package must never give.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

//go:embed prices.json
var bakedPrices []byte

const (
	// remoteURL is LiteLLM's price table — the one machine-readable source that
	// tracks every provider's rates (Anthropic publishes none), kept current by a
	// project with its own reason to care. Fetched at most once per unknown model
	// per run, never on a schedule.
	remoteURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

	// httpTimeout bounds the refresh; bodyLimit bounds what is read from it (the
	// table is ~2 MiB and grows with the industry, so the cap is slack).
	httpTimeout = 15 * time.Second
	bodyLimit   = 16 << 20

	// cacheFile holds the refreshed table under the lich config dir.
	cacheFile = "prices.json"
)

// Rate is one model's price per token, in USD, split the way a provider bills
// them: fresh input, generated output, a cache hit, and writing the cache.
type Rate struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// Tokens is a turn's (or a whole conversation's) token counts, named after the
// four counters a provider transcript reports.
type Tokens struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
}

// Cost is what these tokens cost at this rate, in USD.
func (r Rate) Cost(t Tokens) float64 {
	return r.Input*float64(t.Input) +
		r.Output*float64(t.Output) +
		r.CacheRead*float64(t.CacheRead) +
		r.CacheWrite*float64(t.CacheWrite)
}

// Table resolves model ids to rates. The zero value is not usable — call New.
type Table struct {
	mu    sync.RWMutex
	rates map[string]Rate
	// asked records the model ids a refresh has already been spent on, so a model
	// that stays unknown (a local proxy, a private deployment) costs one request
	// per run and not one per turn.
	asked map[string]bool
	// refreshing is held for the duration of a refresh so concurrent misses share
	// the one in-flight request.
	refreshing bool

	http      *http.Client
	url       string
	cachePath string
}

// New returns a table backed by the baked prices, overlaid with the cached
// refresh if one was written by an earlier run.
func New() *Table {
	return newTable(remoteURL, cachePath(), &http.Client{Timeout: httpTimeout})
}

// newTable builds a table against a given source and cache location — the seam
// the suite drives instead of the real network. A missing or corrupt cache is
// not an error: the baked floor answers alone.
func newTable(url, cachePath string, client *http.Client) *Table {
	t := &Table{
		rates:     parseRates(bakedPrices),
		asked:     make(map[string]bool),
		http:      client,
		url:       url,
		cachePath: cachePath,
	}
	if data, err := os.ReadFile(cachePath); err == nil {
		t.merge(parseRates(data))
	}
	return t
}

// Rate returns the model's price per token. ok is false for a model the table
// cannot price — the caller must report no cost at all, never a zero one — and
// schedules the one refresh that may teach it, so the next turn can answer.
func (t *Table) Rate(model string) (Rate, bool) {
	if model == "" {
		return Rate{}, false
	}
	if rate, ok := t.lookup(model); ok {
		return rate, true
	}
	go t.refreshFor(model)
	return Rate{}, false
}

// lookup resolves a model id against the table: the id as reported, then with a
// dated suffix stripped ("claude-opus-4-5-20251101" → "claude-opus-4-5"), which
// is how a provider names one model in two places.
func (t *Table) lookup(model string) (Rate, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if rate, ok := t.rates[model]; ok {
		return rate, true
	}
	rate, ok := t.rates[undated(model)]
	return rate, ok
}

// datedSuffix matches the "-20251101" a provider appends to pin a model release.
var datedSuffix = regexp.MustCompile(`-\d{8}$`)

func undated(model string) string {
	return datedSuffix.ReplaceAllString(model, "")
}

// refreshFor fetches the remote table once for a model the baked floor and the
// cache both miss. Every failure is silent past a debug line: the caller has
// already reported "unknown", which is the honest answer whether or not this
// works.
func (t *Table) refreshFor(model string) {
	if !t.claimRefresh(model) {
		return
	}
	defer t.releaseRefresh()

	rates, err := t.fetch()
	if err != nil {
		slog.Debug("pricing: refresh failed", "model", model, "err", err)
		return
	}
	t.merge(rates)
	t.writeCache(rates)
}

// claimRefresh reports whether this miss should run the refresh: it must be the
// first miss for that model, and no other refresh may be in flight.
func (t *Table) claimRefresh(model string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.asked[model] || t.refreshing {
		return false
	}
	t.asked[model] = true
	t.refreshing = true
	return true
}

func (t *Table) releaseRefresh() {
	t.mu.Lock()
	t.refreshing = false
	t.mu.Unlock()
}

// merge overlays newer rates on the table. Remote wins: it is the layer that
// exists because the baked one went stale.
func (t *Table) merge(rates map[string]Rate) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for model, rate := range rates {
		t.rates[model] = rate
	}
}

// fetch downloads the remote table and reduces it to lich's shape — every entry
// that carries a real price, whatever provider it belongs to.
func (t *Table) fetch() (map[string]Rate, error) {
	req, err := http.NewRequest(http.MethodGet, t.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "lich")
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		return nil, err
	}
	rates, err := parseRemote(data)
	if err != nil {
		return nil, err
	}
	if len(rates) == 0 {
		return nil, fmt.Errorf("no priced models in the remote table")
	}
	return rates, nil
}

// remoteEntry is the subset of a LiteLLM table entry lich reads. Its other forty
// fields are ignored, so a change to any of them cannot break this.
type remoteEntry struct {
	Input      float64 `json:"input_cost_per_token"`
	Output     float64 `json:"output_cost_per_token"`
	CacheRead  float64 `json:"cache_read_input_token_cost"`
	CacheWrite float64 `json:"cache_creation_input_token_cost"`
}

// parseRemote converts the remote table to lich's shape, dropping every entry
// priced at nothing. A free model and a model whose price nobody filled in look
// identical here, and "unknown" is the safe reading of both.
func parseRemote(data []byte) (map[string]Rate, error) {
	var raw map[string]remoteEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	rates := make(map[string]Rate, len(raw))
	for model, entry := range raw {
		if entry.Input == 0 && entry.Output == 0 {
			continue
		}
		rates[model] = Rate{
			Input:      entry.Input,
			Output:     entry.Output,
			CacheRead:  entry.CacheRead,
			CacheWrite: entry.CacheWrite,
		}
	}
	return rates, nil
}

// parseRates reads lich's own shape (the baked file and the cache). Unparsable
// content yields an empty table rather than an error: both callers treat "no
// rates from this layer" as the miss it is.
func parseRates(data []byte) map[string]Rate {
	rates := make(map[string]Rate)
	if err := json.Unmarshal(data, &rates); err != nil {
		slog.Debug("pricing: unreadable price table", "err", err)
		return make(map[string]Rate)
	}
	return rates
}

// writeCache saves the refreshed table so the next run starts already knowing
// the models this one had to ask about. Best effort — an unwritable config dir
// costs one request per run, nothing more.
func (t *Table) writeCache(rates map[string]Rate) {
	if t.cachePath == "" {
		return
	}
	data, err := json.Marshal(rates)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(t.cachePath), 0o755); err != nil {
		slog.Debug("pricing: cache directory", "err", err)
		return
	}
	if err := os.WriteFile(t.cachePath, data, 0o644); err != nil {
		slog.Debug("pricing: write cache", "err", err)
	}
}

// cachePath is <config-dir>/lich/prices.json, or "" when the OS has no config
// dir to offer — the cache is then skipped and the baked floor stands.
func cachePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "lich", cacheFile)
}

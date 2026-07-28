package pricing

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// remoteBody is the shape LiteLLM publishes, cut to the fields lich reads: two
// priced models, one free (a placeholder nobody filled in), and the noise field
// every real entry carries.
const remoteBody = `{
  "claude-opus-9": {
    "input_cost_per_token": 1e-05,
    "output_cost_per_token": 5e-05,
    "cache_read_input_token_cost": 1e-06,
    "cache_creation_input_token_cost": 1.25e-05,
    "litellm_provider": "anthropic",
    "supports_pdf_input": true
  },
  "gpt-9": {
    "input_cost_per_token": 2e-06,
    "output_cost_per_token": 8e-06,
    "litellm_provider": "openai"
  },
  "sample_spec": {
    "input_cost_per_token": 0.0,
    "output_cost_per_token": 0.0
  }
}`

// serveRemote stands in for the published table and counts what asked for it.
func serveRemote(t *testing.T, body string) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// TestBakedTablePricesAShippedModel proves the floor answers with no network
// and no cache: a fresh install prices the models it shipped knowing.
func TestBakedTablePricesAShippedModel(t *testing.T) {
	table := newTable("http://0.0.0.0:0", filepath.Join(t.TempDir(), "prices.json"), http.DefaultClient)

	rate, ok := table.Rate("claude-opus-4-5")
	if !ok {
		t.Fatal("Rate(claude-opus-4-5): want a price from the baked table")
	}
	if rate.Input != 5e-06 || rate.Output != 2.5e-05 {
		t.Errorf("Rate = %+v, want the published opus-4-5 rates", rate)
	}
}

// TestDatedModelIdResolvesToItsFamily proves the id a transcript records
// ("…-20251101", pinning a release) prices off the undated entry. Without it
// every dated id would read as unknown and the readout would never appear.
func TestDatedModelIdResolvesToItsFamily(t *testing.T) {
	table := newTable("http://0.0.0.0:0", filepath.Join(t.TempDir(), "prices.json"), http.DefaultClient)

	// A date the baked table has no exact entry for, so only the strip can
	// answer — the shape a model release takes between two lich builds.
	dated, ok := table.Rate("claude-sonnet-4-5-20260714")
	if !ok {
		t.Fatal("Rate: a dated id must resolve to its family")
	}
	undatedRate, _ := table.Rate("claude-sonnet-4-5")
	if dated != undatedRate {
		t.Errorf("dated rate = %+v, want the same as undated %+v", dated, undatedRate)
	}
}

// TestUnknownModelIsUnpricedNotFree is the invariant the whole feature rests
// on: a model the table has never heard of reports no price, so the caller
// shows no number. Pricing it at zero would render a running total of $0.00
// over a session that cost real money.
func TestUnknownModelIsUnpricedNotFree(t *testing.T) {
	srv, _ := serveRemote(t, remoteBody)
	table := newTable(srv.URL, filepath.Join(t.TempDir(), "prices.json"), srv.Client())

	if rate, ok := table.Rate("some-private-deployment"); ok {
		t.Errorf("Rate = %+v, ok=true; want an unpriced miss", rate)
	}
	if rate, ok := table.Rate(""); ok {
		t.Errorf("Rate(\"\") = %+v, ok=true; want an unpriced miss", rate)
	}
}

// TestRefreshTeachesAModelReleasedAfterTheBuild is the reason the remote layer
// exists: a model that ships after this binary did prices correctly once the
// refresh lands, without a release of lich.
func TestRefreshTeachesAModelReleasedAfterTheBuild(t *testing.T) {
	srv, hits := serveRemote(t, remoteBody)
	table := newTable(srv.URL, filepath.Join(t.TempDir(), "prices.json"), srv.Client())

	// lookup, not Rate: Rate's miss starts the refresh itself, and asserting on
	// the synchronous one below while that goroutine is still running is how a
	// test like this goes flaky.
	if _, ok := table.lookup("claude-opus-9"); ok {
		t.Fatal("lookup: the baked table cannot know a model published after it")
	}
	table.refreshFor("claude-opus-9")

	rate, ok := table.Rate("claude-opus-9")
	if !ok {
		t.Fatal("Rate after refresh: want the freshly published price")
	}
	if rate.Input != 1e-05 || rate.CacheWrite != 1.25e-05 {
		t.Errorf("Rate = %+v, want the remote opus-9 rates", rate)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("remote hits = %d, want exactly 1", got)
	}
}

// TestRefreshKeepsEveryProvidersPrices proves the table stays provider-agnostic:
// the reader is Claude-only today, but the prices a second reader will need are
// already there — nothing filters the remote table down to one vendor.
func TestRefreshKeepsEveryProvidersPrices(t *testing.T) {
	srv, _ := serveRemote(t, remoteBody)
	table := newTable(srv.URL, filepath.Join(t.TempDir(), "prices.json"), srv.Client())

	table.refreshFor("gpt-9")

	if _, ok := table.Rate("gpt-9"); !ok {
		t.Error("Rate(gpt-9): a non-Claude model must price from the same table")
	}
}

// TestAZeroPricedEntryIsDropped pins the read of an unpriced remote entry: the
// documentation placeholder and a model nobody filled in both cost 0.0 there,
// and "unknown" is the only safe reading of both.
func TestAZeroPricedEntryIsDropped(t *testing.T) {
	srv, _ := serveRemote(t, remoteBody)
	table := newTable(srv.URL, filepath.Join(t.TempDir(), "prices.json"), srv.Client())

	table.refreshFor("sample_spec")

	if rate, ok := table.Rate("sample_spec"); ok {
		t.Errorf("Rate(sample_spec) = %+v, ok=true; want it dropped as unpriced", rate)
	}
}

// TestAModelIsOnlyAskedAboutOnce proves the miss path cannot turn into a
// request per turn: emitUsage runs on every non-idle hook, and a model that
// stays unknown (a proxy, a private deployment) must cost one request per run.
func TestAModelIsOnlyAskedAboutOnce(t *testing.T) {
	srv, hits := serveRemote(t, remoteBody)
	table := newTable(srv.URL, filepath.Join(t.TempDir(), "prices.json"), srv.Client())

	table.refreshFor("still-unknown")
	table.refreshFor("still-unknown")
	table.refreshFor("still-unknown")

	if got := hits.Load(); got != 1 {
		t.Errorf("remote hits = %d, want exactly 1", got)
	}
}

// TestTheRefreshIsCachedForTheNextRun proves the fetched table survives a
// restart: the second run answers for a post-build model with the server gone.
func TestTheRefreshIsCachedForTheNextRun(t *testing.T) {
	srv, _ := serveRemote(t, remoteBody)
	cache := filepath.Join(t.TempDir(), "cache", "prices.json")

	first := newTable(srv.URL, cache, srv.Client())
	first.refreshFor("claude-opus-9")

	next := newTable("http://0.0.0.0:0", cache, http.DefaultClient)
	if _, ok := next.Rate("claude-opus-9"); !ok {
		t.Error("Rate: the cached refresh must answer without the network")
	}
}

// TestACorruptCacheLeavesTheBakedFloorStanding proves a half-written cache
// cannot take the readout down with it — the layer below still answers.
func TestACorruptCacheLeavesTheBakedFloorStanding(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "prices.json")
	if err := os.WriteFile(cache, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	table := newTable("http://0.0.0.0:0", cache, http.DefaultClient)

	if _, ok := table.Rate("claude-opus-4-5"); !ok {
		t.Error("Rate: a corrupt cache must not shadow the baked prices")
	}
}

// TestAFailedRefreshChangesNothing pins the offline path: an unreachable or
// broken source leaves the table exactly as it was, and the caller keeps
// reporting "unknown" rather than a guess.
func TestAFailedRefreshChangesNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	table := newTable(srv.URL, filepath.Join(t.TempDir(), "prices.json"), srv.Client())

	table.refreshFor("claude-opus-9")

	if _, ok := table.Rate("claude-opus-9"); ok {
		t.Error("Rate: a failed refresh must not invent a price")
	}
	if _, ok := table.Rate("claude-opus-4-5"); !ok {
		t.Error("Rate: a failed refresh must not lose the prices already held")
	}
}

// TestCostBillsEveryCounterSeparately proves the four counters are priced at
// their own rates — a cache read is an order of magnitude cheaper than fresh
// input, and folding them together would overstate a long session badly.
func TestCostBillsEveryCounterSeparately(t *testing.T) {
	rate := Rate{Input: 1e-05, Output: 5e-05, CacheRead: 1e-06, CacheWrite: 1.25e-05}

	got := rate.Cost(Tokens{Input: 100, Output: 200, CacheRead: 300, CacheWrite: 400})

	want := 100*1e-05 + 200*5e-05 + 300*1e-06 + 400*1.25e-05
	if got != want {
		t.Errorf("Cost = %v, want %v", got, want)
	}
}

// TestEveryBakedRateIsPriced guards the generated table itself: an entry that
// lost its rates in a regeneration would price a session at zero, which is the
// one answer this package must never give.
func TestEveryBakedRateIsPriced(t *testing.T) {
	rates := parseRates(bakedPrices)
	if len(rates) == 0 {
		t.Fatal("the baked table is empty")
	}
	for model, rate := range rates {
		if rate.Input <= 0 || rate.Output <= 0 {
			t.Errorf("baked rate for %q = %+v, want real input and output prices", model, rate)
		}
	}
}

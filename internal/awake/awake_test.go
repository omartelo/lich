package awake

import (
	"errors"
	"testing"
)

// The count is a level, not an edge stream: only the crossings of zero touch
// the assertion, and each crossing touches it exactly once.
func TestKeeperHoldsAcrossCountAndReleasesAtZero(t *testing.T) {
	holds, releases := 0, 0
	k := &Keeper{hold: func() (func(), error) {
		holds++
		return func() { releases++ }, nil
	}}
	for _, n := range []int{1, 2, 3, 2, 1} {
		k.Set(n)
	}
	if holds != 1 || releases != 0 {
		t.Fatalf("after rising: holds=%d releases=%d, want 1/0", holds, releases)
	}
	k.Set(0)
	k.Set(0)
	if holds != 1 || releases != 1 {
		t.Fatalf("after zero twice: holds=%d releases=%d, want 1/1", holds, releases)
	}
	k.Set(1)
	if holds != 2 {
		t.Fatalf("a second burst of work must hold again, holds=%d", holds)
	}
}

// A hold that fails is not remembered as held: the next rise from zero tries
// again, and a zero in between releases nothing.
func TestKeeperRetriesAfterFailedHold(t *testing.T) {
	calls, releases := 0, 0
	k := &Keeper{hold: func() (func(), error) {
		calls++
		if calls == 1 {
			return nil, errors.New("no inhibitor here")
		}
		return func() { releases++ }, nil
	}}
	k.Set(1)
	k.Set(0)
	if releases != 0 {
		t.Fatalf("released a hold that never happened")
	}
	k.Set(1)
	if calls != 2 {
		t.Fatalf("hold calls=%d, want a retry", calls)
	}
}

// The real seam builds and hands out a hold on this OS; whether the OS honours
// it is measured outside the suite (`pmset -g assertions`, `powercfg /requests`).
func TestNewUsesThisOSHold(t *testing.T) {
	if New().hold == nil {
		t.Fatal("New wired no hold")
	}
}

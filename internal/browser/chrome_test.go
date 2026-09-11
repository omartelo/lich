package browser

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestWithPageDeadlineKeepsChromedpTarget(t *testing.T) {
	pageCtx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	if chromedp.FromContext(pageCtx) == nil {
		t.Fatal("NewContext produced no chromedp target")
	}

	wait, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if chromedp.FromContext(wait) != nil {
		t.Fatal("a Background timeout unexpectedly carried a chromedp target")
	}

	got, done := withPageDeadline(pageCtx, wait)
	defer done()
	if chromedp.FromContext(got) == nil {
		t.Fatal("deadline wrapper dropped the chromedp target; CDP would return invalid context")
	}
	if _, ok := got.Deadline(); !ok {
		t.Fatal("deadline wrapper dropped the wait deadline")
	}
}

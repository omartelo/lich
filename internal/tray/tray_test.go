package tray

import "testing"

func TestRunningFillsTheCount(t *testing.T) {
	l := Labels{Running: "Sessions running: {count}"}
	if got := l.running(4); got != "Sessions running: 4" {
		t.Fatalf("running(4) = %q", got)
	}
}

func TestSetLabelsStartsOnceThenRelabels(t *testing.T) {
	starts := 0
	var relabeled []Labels
	tr := New(Icons{}, func() {}, func() error { return nil }, func() int { return 0 })
	tr.start = func(t *Tray) {
		starts++
		t.setRelabel(func(l Labels) { relabeled = append(relabeled, l) })
	}

	en := Labels{Show: "Show lich", Running: "Sessions running: {count}", Quit: "Quit lich"}
	pt := Labels{Show: "Mostrar o lich", Running: "Sessões rodando: {count}", Quit: "Encerrar o lich"}
	tr.SetLabels(en)
	if starts != 1 || len(relabeled) != 0 {
		t.Fatalf("first SetLabels: starts=%d relabeled=%v, want the tray put up once", starts, relabeled)
	}
	if tr.currentLabels() != en {
		t.Fatalf("labels = %+v, want %+v", tr.currentLabels(), en)
	}
	tr.SetLabels(pt)
	if starts != 1 || len(relabeled) != 1 || relabeled[0] != pt {
		t.Fatalf("second SetLabels: starts=%d relabeled=%v, want the menu rewritten in place", starts, relabeled)
	}
}

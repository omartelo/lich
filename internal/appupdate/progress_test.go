package appupdate

import (
	"io"
	"strings"
	"testing"
	"time"
)

// collectProgress returns a Service whose emit appends every step to the
// returned slice.
func collectProgress() (*Service, *[]Progress) {
	steps := &[]Progress{}
	s := &Service{emit: func(name string, data any) {
		if name != ProgressEventName {
			panic("unexpected event " + name)
		}
		*steps = append(*steps, data.(Progress))
	}}
	return s, steps
}

func TestProgressReaderStepsByPercent(t *testing.T) {
	s, steps := collectProgress()
	body := strings.Repeat("x", 1000)
	r := s.progressReader(strings.NewReader(body), 1000, phaseInstall)
	if got, err := io.ReadAll(iotest5(r)); err != nil || len(got) != 1000 {
		t.Fatalf("read = %d bytes, %v", len(got), err)
	}
	// Announced at 0, then one step per 1% (every second 5-byte read), then
	// the phase that follows the download.
	first, last := (*steps)[0], (*steps)[len(*steps)-1]
	if first != (Progress{Phase: phaseDownload, Received: 0, Total: 1000}) {
		t.Errorf("first step = %+v", first)
	}
	if last != (Progress{Phase: phaseInstall}) {
		t.Errorf("last step = %+v", last)
	}
	if done := (*steps)[len(*steps)-2]; done != (Progress{Phase: phaseDownload, Received: 1000, Total: 1000}) {
		t.Errorf("final download step = %+v", done)
	}
	if n := len(*steps); n < 100 || n > 103 {
		t.Errorf("steps = %d, want one per percent", n)
	}
}

func TestProgressReaderUnknownTotalStepsByTime(t *testing.T) {
	s, steps := collectProgress()
	now := time.Unix(0, 0)
	p := &progressReader{r: strings.NewReader(strings.Repeat("x", 30)), total: -1, after: phaseInstaller, emit: s.emitProgress, now: func() time.Time { return now }}
	p.report()
	buf := make([]byte, 10)
	if _, err := p.Read(buf); err != nil {
		t.Fatal(err)
	}
	if len(*steps) != 1 {
		t.Fatalf("a read inside the interval must not report, steps = %+v", *steps)
	}
	now = now.Add(progressInterval)
	if _, err := p.Read(buf); err != nil {
		t.Fatal(err)
	}
	if got := (*steps)[1]; got != (Progress{Phase: phaseDownload, Received: 20, Total: -1}) {
		t.Errorf("timed step = %+v", got)
	}
	if _, err := io.ReadAll(p); err != nil {
		t.Fatal(err)
	}
	if last := (*steps)[len(*steps)-1]; last != (Progress{Phase: phaseInstaller}) {
		t.Errorf("last step = %+v", last)
	}
}

func TestProgressSilentWithoutHub(t *testing.T) {
	s := &Service{}
	r := s.progressReader(strings.NewReader("abc"), 3, phaseInstall)
	if got, err := io.ReadAll(r); err != nil || string(got) != "abc" {
		t.Fatalf("read = %q, %v", got, err)
	}
}

// iotest5 reads its source five bytes at a time, so a percent of a
// 1000-byte body takes two reads.
func iotest5(r io.Reader) io.Reader { return &chunked{r: r, n: 5} }

type chunked struct {
	r io.Reader
	n int
}

func (c *chunked) Read(b []byte) (int, error) {
	if len(b) > c.n {
		b = b[:c.n]
	}
	return c.r.Read(b)
}

package appupdate

import (
	"io"
	"time"
)

// ProgressEventName is the hub event Apply's phases ride on, for the toast.
const ProgressEventName = "appupdate-progress"

// Progress is one step of Apply: bytes received of total during the download
// (total is -1 when the asset came without a Content-Length), then the phase
// that has no percentage — "install" for the in-place swap, "installer" where
// lich hands over to the Windows installer and closes.
type Progress struct {
	Phase    string `json:"phase"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
}

const (
	phaseDownload  = "download"
	phaseInstall   = "install"
	phaseInstaller = "installer"
	// A download event per ~1% or per 250 ms, whichever comes first: enough for
	// the bar to move on a slow link, and no repaint per chunk on a fast one.
	progressStepPct  = 1
	progressInterval = 250 * time.Millisecond
)

// progressReader counts the asset body as it passes and reports it, then
// announces the phase that follows the download when the body ends.
type progressReader struct {
	r        io.Reader
	total    int64
	after    string
	emit     func(Progress)
	now      func() time.Time
	received int64
	lastN    int64
	lastAt   time.Time
}

func (s *Service) progressReader(r io.Reader, total int64, after string) io.Reader {
	p := &progressReader{r: r, total: total, after: after, emit: s.emitProgress, now: time.Now}
	p.report()
	return p
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.received += int64(n)
	if err == io.EOF {
		p.report()
		p.emit(Progress{Phase: p.after})
		return n, err
	}
	if p.due() {
		p.report()
	}
	return n, err
}

func (p *progressReader) due() bool {
	if p.total > 0 && (p.received-p.lastN)*100/p.total >= progressStepPct {
		return true
	}
	return p.now().Sub(p.lastAt) >= progressInterval
}

func (p *progressReader) report() {
	p.lastN, p.lastAt = p.received, p.now()
	p.emit(Progress{Phase: phaseDownload, Received: p.received, Total: p.total})
}

// emitProgress hands one step to the hub; a Service built without one (tests,
// a bare binary) updates in silence.
func (s *Service) emitProgress(p Progress) {
	if s.emit != nil {
		s.emit(ProgressEventName, p)
	}
}

// Package rage collects what a lich bug report needs into one archive: the
// facts that decide whether a report is actionable (version, platform, browser,
// provider and plugin state), the environment lich was launched with, and the
// log generations — with the loopback token masked wherever it appeared.
//
// It exists for the failure lich cannot report on itself: no window means no
// Settings › Help, so this runs from the command line and needs no running
// instance. Nothing is uploaded and nothing is filed — the archive stays on
// disk until the user attaches it to an issue they wrote themselves.
package rage

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/chromium"
	"github.com/omartelo/lich/internal/logging"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/singleton"
)

// maxLogBytes bounds each log generation in the archive. Rotation caps the file
// at startup only, so a long-running session at debug level can outgrow it
// while running — such a file is carried by its tail, which is the half that
// holds what just went wrong.
const maxLogBytes = 4 << 20

// Report is what report.md renders: every fact gathered about this machine,
// and none that identify the loopback session. The token is deliberately
// absent — the file outlives the instance it would name.
type Report struct {
	Version   string
	GoVersion string
	// Revision is the VCS stamp Go embeds when a build comes from a checkout,
	// suffixed "-dirty" for a modified tree. It is what identifies a build the
	// reporter compiled themselves, where Version says only "dev".
	Revision  string
	Platform  string
	Instance  string
	ConfigDir string
	LogPath   string
	Browser   string
	Providers []providers.Detected
	Plugin    []agentplugin.Status
	Entries   []Entry
}

// Entry is one item in the lich config directory, reported by name and size so
// a missing database or an empty theme folder is visible without shipping any
// of their contents.
type Entry struct {
	Name string
	Size int64
	Dir  bool
}

// Collector gathers a Report. The five probes are fields because each one
// reaches the machine — PATH, the provider CLIs, the network, the running
// instance — and a test must answer for all of them.
type Collector struct {
	configDir string
	version   string
	environ   []string

	browser func() (string, error)
	detect  func() []providers.Detected
	plugin  func() []agentplugin.Status
	probe   func() (*singleton.Info, bool)
	dump    func(port int, token string) ([]byte, error)
}

// New returns a Collector reading the real machine. configDir is the OS config
// dir (os.UserConfigDir), the parent of lich's own directory, because that is
// what the runtime file and the log path are both resolved from.
func New(configDir, version string, environ []string) *Collector {
	return &Collector{
		configDir: configDir,
		version:   version,
		environ:   environ,
		browser:   func() (string, error) { return chromium.FindBrowser(exec.LookPath) },
		detect: func() []providers.Detected {
			found, err := providers.New().Detect()
			if err != nil {
				return nil
			}
			return found
		},
		plugin: func() []agentplugin.Status { return agentplugin.New(pathBins{}).Status() },
		probe: func() (*singleton.Info, bool) {
			info, err := singleton.Read(configDir)
			if err != nil || info == nil {
				return nil, false
			}
			return info, singleton.Ping(info.Port, info.Token)
		},
		dump: fetchGoroutines,
	}
}

// dumpTimeout bounds the goroutine fetch. A lich that stopped answering is the
// case this runs in, so the wait has to end on its own: what the file then
// carries is that it did not answer, which is the finding.
const dumpTimeout = 5 * time.Second

// maxDumpBytes bounds the dump the bundle carries. A stack per goroutine stays
// far below this; a runaway instance with tens of thousands of them is
// diagnosed by the head of the file, not by its tail.
const maxDumpBytes = 8 << 20

// fetchGoroutines asks the running instance for its goroutine stacks (the
// transport's /debug/goroutines).
func fetchGoroutines(port int, token string) ([]byte, error) {
	client := http.Client{Timeout: dumpTimeout}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/goroutines?token=%s", port, token))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxDumpBytes))
}

// pathBins resolves every provider to its default name on PATH. The real
// resolver is the workspace database, and rage never opens it: the database
// belongs to the running instance, and a bug report must not be the second
// process to migrate it.
type pathBins struct{}

func (pathBins) ProviderBin(_, _ string) string { return "" }

// Bundle writes the gzipped tar to w and returns the report it carries. root is
// the directory every entry sits under, so extracting two bundles side by side
// does not merge them.
func (c *Collector) Bundle(w io.Writer, root string) (Report, error) {
	report, live := c.collect()
	token := ""
	if live != nil {
		token = live.Token
	}

	gz := gzip.NewWriter(w)
	archive := tar.NewWriter(gz)

	files := []bundleFile{
		{"report.md", []byte(render(report))},
		{"env.txt", []byte(renderEnv(c.environ, token))},
	}
	files = append(files, logFiles(report.LogPath, token)...)
	if live != nil {
		files = append(files, bundleFile{"goroutines.txt", c.goroutines(live)})
	}

	for _, f := range files {
		if err := writeEntry(archive, root+"/"+f.name, f.data); err != nil {
			return report, err
		}
	}
	if err := archive.Close(); err != nil {
		return report, fmt.Errorf("close the archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return report, fmt.Errorf("compress the archive: %w", err)
	}
	return report, nil
}

// bundleFile is one entry of the archive, held in memory: the whole bundle is
// two rendered pages and at most two capped log generations.
type bundleFile struct {
	name string
	data []byte
}

// logFiles reads the log generations the archive carries — the live one and the
// rotated *.old, which is often the one holding the crash. An unreadable
// generation is skipped rather than failing the bundle: a report missing one log
// still beats no report.
func logFiles(logPath, token string) []bundleFile {
	if logPath == "" {
		return nil
	}
	var out []bundleFile
	for _, path := range []string{logPath, logPath + ".old"} {
		data, err := readTail(path, maxLogBytes)
		if err != nil {
			continue
		}
		out = append(out, bundleFile{"logs/" + filepath.Base(path), scrub(data, token)})
	}
	return out
}

// goroutines is the running instance's stacks, or — when it will not answer —
// the record that it did not. The failure is the point: a lich still holding
// its port with a window that stopped updating leaves nothing in the log, and
// "did not answer in 5s" is already half the diagnosis.
func (c *Collector) goroutines(live *singleton.Info) []byte {
	data, err := c.dump(live.Port, live.Token)
	if err != nil {
		return []byte(fmt.Sprintf(
			"the running lich (pid %d, port %d) did not answer the goroutine dump: %v\n", live.PID, live.Port, err))
	}
	return scrub(data, live.Token)
}

// collect gathers every fact, and returns the running instance beside the
// report rather than inside it: its token is needed to mask the copies of
// itself that may sit in the log, and its port to reach it for the goroutine
// dump. Nil when nothing is running or the recorded instance does not answer —
// the report says which.
func (c *Collector) collect() (Report, *singleton.Info) {
	lichDir := filepath.Join(c.configDir, "lich")
	report := Report{
		Version:   c.version,
		GoVersion: runtime.Version(),
		Revision:  revision(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		ConfigDir: lichDir,
		LogPath:   logging.Path(lichDir),
		Providers: c.detect(),
		Plugin:    c.plugin(),
		Entries:   entries(lichDir),
	}
	if _, err := os.Stat(report.LogPath); err != nil {
		// A path that was never written would send the reporter looking for a
		// file that does not exist, and the archive carries no logs section.
		report.LogPath = ""
	}

	browser, err := c.browser()
	report.Browser = browser
	if err != nil {
		report.Browser = err.Error()
	}

	info, alive := c.probe()
	switch {
	case info == nil:
		report.Instance = "not running"
	case alive:
		report.Instance = fmt.Sprintf("running (pid %d, port %d)", info.PID, info.Port)
		return report, info
	default:
		// The file outlives a crash, so a recorded instance that does not answer
		// is itself a finding: lich died without closing its window.
		report.Instance = fmt.Sprintf("recorded but not answering (pid %d, port %d)", info.PID, info.Port)
	}
	return report, nil
}

// entries lists the lich config directory one level deep. Directories are named
// but never descended into: below them are the Chromium profile's cookies and
// the workspace database, neither of which belongs in a bug report.
func entries(dir string) []Entry {
	found, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]Entry, 0, len(found))
	for _, e := range found {
		entry := Entry{Name: e.Name(), Dir: e.IsDir()}
		if info, err := e.Info(); err == nil {
			entry.Size = info.Size()
		}
		out = append(out, entry)
	}
	return out
}

// readTail reads the last max bytes of a file, marking the cut so a reader does
// not take a mid-line start for a corrupt log.
func readTail(path string, max int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() <= max {
		return io.ReadAll(file)
	}
	if _, err := file.Seek(info.Size()-max, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	head := fmt.Sprintf("[lich rage: %d earlier bytes omitted, this is the tail]\n", info.Size()-max)
	return append([]byte(head), data...), nil
}

// writeEntry adds one file to the archive, 0600 like everything else lich
// writes: the bundle carries paths and project names.
func writeEntry(archive *tar.Writer, name string, data []byte) error {
	header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(data))}
	if err := archive.WriteHeader(header); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	if _, err := archive.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// DefaultBase is the bundle's name without an extension: the archive file is
// this plus ".tar.gz", and the directory inside it is this exactly.
func DefaultBase(now time.Time) string {
	return "lich-rage-" + now.Format("20060102-150405")
}

// revision reads the VCS stamp Go embeds at build time, empty when the binary
// was not built from a checkout.
func revision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev string
	dirty := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			rev = setting.Value
		case "vcs.modified":
			dirty = setting.Value == "true"
		}
	}
	if rev != "" && dirty {
		return rev + "-dirty"
	}
	return rev
}

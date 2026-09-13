package agentplugin

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/relay"
)

// Five harnesses keep lich's MCP server as a line in a file naming this binary's
// absolute path, written once at install. Moving the binary (a package that
// changed its install path, an update, a `go run` cache that was swept) leaves
// every one of those registrations pointing at nothing, and the harness drops the
// tools without a word. So a starting lich walks them and points each back at
// itself, through the same write the install makes.
//
// Only a registration that is there is touched: one the user removed stays gone,
// and so does one they disabled.

// registration is one harness's stored lich server: where it is read from and
// how the install rewrites it.
type registration struct {
	provider string
	// command is the binary the registration names, and false when there is no
	// live lich entry to repair.
	command func() (string, bool)
	// rewrite points the registration at lichBin through the install's own write.
	rewrite func(lichBin string) error
}

func (s *Service) registrations() []registration {
	return []registration{
		{providers.Crush, s.crushRegisteredCommand, s.crushRewrite},
		{providers.OMP, documentCommand(ompMCPPath), documentRewrite(ompMCPPath)},
		{providers.Cursor, documentCommand(cursorMCPPath), documentRewrite(cursorMCPPath)},
		{providers.Kiro, documentCommand(providers.KiroAgentPath), func(string) error { return s.kiroRegisterMCP() }},
		{providers.Antigravity, documentCommand(antigravityMCPPath), func(string) error { return s.antigravityRegisterMCP() }},
	}
}

// RepairRegistrations rewrites every lich MCP registration that names a binary
// other than this one. It runs once at startup, off the prompt: Crush's config
// directory and two of the rewrites are CLI calls.
func (s *Service) RepairRegistrations() {
	exe, err := os.Executable()
	if devInstance(os.Getenv(devEnv), exe, err) {
		return
	}
	lichBin := s.lichBin()
	if lichBin == "" {
		return
	}
	for _, r := range s.registrations() {
		if err := s.repair(r, lichBin); err != nil {
			slog.Warn("agentplugin: repair MCP registration", "provider", r.provider, "err", err)
		}
	}
}

// devEnv is what `task dev` sets to give its lich a database of its own.
const devEnv = "LICH_DEV"

// devInstance reports whether this lich is a development rig. One shares the
// installed lich's home, so repairing from it would repoint the user's real
// registrations at a dev binary, and the two would swap them back on every start.
func devInstance(devFlag, exe string, exeErr error) bool {
	return devFlag != "" || (exeErr == nil && underGoBuildCache(exe))
}

func (s *Service) repair(r registration, lichBin string) error {
	registered, ok := r.command()
	if !ok || sameBinary(registered, lichBin) {
		return nil
	}
	if r.provider == providers.Kiro || r.provider == providers.Antigravity {
		if !s.available(r.provider) {
			return fmt.Errorf("%s is not on PATH to rewrite through", s.bin(r.provider))
		}
	}
	if err := r.rewrite(lichBin); err != nil {
		return err
	}
	slog.Info("agentplugin: repointed MCP registration", "provider", r.provider, "from", registered, "to", lichBin)
	return nil
}

// sameBinary treats a symlink and its target as one binary, so a registration
// naming a stable link (/usr/local/bin/lich) is not swapped for the versioned
// path behind it, which is the one that moves on the next update.
func sameBinary(a, b string) bool {
	if a == b {
		return true
	}
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ai, bi)
}

// documentCommand reads lich's entry out of an `mcpServers` document. Kiro's
// agent file and Antigravity's mcp_config.json hold the same shape their CLIs
// write, so one reader serves all four.
func documentCommand(path func() (string, error)) func() (string, bool) {
	return func() (string, bool) {
		p, err := path()
		if err != nil {
			return "", false
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return "", false
		}
		var doc struct {
			Servers map[string]struct {
				Command  string `json:"command"`
				Disabled bool   `json:"disabled"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return "", false
		}
		entry, ok := doc.Servers[relay.MCPServerName]
		if !ok || entry.Disabled || entry.Command == "" {
			return "", false
		}
		return entry.Command, true
	}
}

func documentRewrite(path func() (string, error)) func(string) error {
	return func(lichBin string) error {
		p, err := path()
		if err != nil {
			return err
		}
		return writeMCPDocument(p, lichBin)
	}
}

// crushRegisteredCommand reads the `--command` of the `mcp add` line inside
// lich's crushrc block. The value is what shquote.Quote wrote, so undoing that
// one escape is the whole parse.
func (s *Service) crushRegisteredCommand() (string, bool) {
	path, err := s.crushrcPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	block := string(data)
	start := indexOfOpenMarker(block)
	if start < 0 {
		return "", false
	}
	block = block[start:]
	if end := strings.Index(block, blockClose); end >= 0 {
		block = block[:end]
	}
	prefix := "mcp add " + relay.MCPServerName + " "
	for line := range strings.Lines(block) {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		_, rest, ok := strings.Cut(line, " --command ")
		quoted, _, _ := strings.Cut(rest, " --args ")
		if !ok || len(quoted) < 2 {
			return "", false
		}
		return strings.ReplaceAll(quoted[1:len(quoted)-1], `'\''`, "'"), true
	}
	return "", false
}

// crushRewrite rewrites lich's block at the version it already carries, so the
// repair moves the binary and nothing else: no release lookup, no new scripts.
func (s *Service) crushRewrite(lichBin string) error {
	version, ok := s.crushInstalledVersion()
	if !ok {
		return fmt.Errorf("crushrc block carries no version")
	}
	dir, err := pluginScriptDir(providers.Crush)
	if err != nil {
		return err
	}
	return s.writeCrushrc(version, dir, lichBin)
}

// ompMCPPath is the document omp reads its servers from.
func ompMCPPath() (string, error) {
	dir, err := ompAgentDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ompMCPFile), nil
}

// antigravityMCPPath is where `agy mcp add` keeps its servers: beside the
// plugins directory, under the same home-only root (measured on 1.1.19).
func antigravityMCPPath() (string, error) {
	dir, err := antigravityPluginDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(filepath.Dir(dir)), "mcp_config.json"), nil
}

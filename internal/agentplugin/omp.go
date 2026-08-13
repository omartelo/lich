package agentplugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/relay"
)

// The oh-my-pi side. omp has no plugin CLI either, and its extensions are
// modules it imports from a directory it scans unprompted — so, as with
// opencode, installing one is writing the file and the file being there is the
// install. omp keeps no plugin state to ask, so the marker line lich writes
// above the module is again the only record of what version is installed.
//
// The MCP registration is the part that differs. omp takes no `--mcp-config`
// flag, and the only place it reads a server from is an `mcpServers` document
// beside the extensions — so lich writes one, merging into whatever is already
// there. It is the same trade Crush's crushrc block makes, one step worse: JSON
// has no comment to hide a marker in and no way to append, so lich rewrites the
// document. Every key survives the round trip; the user's formatting does not.

const (
	// ompSource is the module's path inside the plugin repository, and ompFile
	// the name it takes in omp's extensions directory.
	ompSource = "omp/lich.js"
	ompFile   = "lich.js"

	// ompMCPFile is the document omp reads MCP servers from, in the same agent
	// directory as the extensions.
	ompMCPFile = "mcp.json"

	// ompMCPServers is the key omp reads servers under — the Claude Desktop
	// shape, which omp adopted wholesale.
	ompMCPServers = "mcpServers"
)

func (s *Service) ompInstall() error {
	version, err := s.releaseVersion()
	if err != nil {
		return err
	}
	data, err := s.fetchFile(version, ompSource)
	if err != nil {
		return err
	}
	dir, err := ompAgentDir()
	if err != nil {
		return err
	}
	// Merged before anything is written, because the only way this step fails is
	// a document lich must not replace — and an install that leaves an extension
	// behind while refusing the registration is worse than one the user retries.
	registration, err := ompMCPDocument(dir, lichBinary())
	if err != nil {
		return err
	}
	extensions := filepath.Join(dir, "extensions")
	if err := os.MkdirAll(extensions, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", extensions, err)
	}
	body := fmt.Sprintf("%s %s v%s — installed by lich, replace with the file from %s\n%s",
		jsComment, markerName, version, marketplaceRepo, data)
	path := filepath.Join(extensions, ompFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if registration == nil {
		return nil
	}
	mcp := filepath.Join(dir, ompMCPFile)
	if err := os.WriteFile(mcp, registration, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", mcp, err)
	}
	return nil
}

// ompInstalledVersion reads the version off the marker line lich wrote, or
// ("", false) when the module is absent, unreadable, or was not written by lich.
func (s *Service) ompInstalledVersion() (string, bool) {
	dir, err := ompAgentDir()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, "extensions", ompFile))
	if err != nil {
		return "", false
	}
	return versionOf(data, jsComment)
}

// ompMCPDocument is omp's `mcpServers` document with lich's server registered in
// it, leaving every other server and every other key exactly as they were. Nil
// when there is nothing to write: a lich that cannot name its own binary
// registers nothing rather than a command that cannot run — the reports are the
// half omp needs to be useful, and they do not depend on it.
//
// The path written is this binary's, resolved now, for the reason Crush's block
// gives: a registration in a file cannot expand a variable per session. It is
// only the transport — which lich a session reaches is decided by the
// coordinates in its PTY.
func ompMCPDocument(dir, lichBin string) ([]byte, error) {
	if lichBin == "" {
		return nil, nil
	}
	path := filepath.Join(dir, ompMCPFile)
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	config := map[string]any{}
	if len(existing) > 0 {
		// Refused rather than replaced: the file belongs to the user, and
		// overwriting one lich failed to understand would delete servers it
		// cannot see.
		if err := json.Unmarshal(existing, &config); err != nil {
			return nil, fmt.Errorf("%s is not a JSON object lich can merge into: %w", path, err)
		}
	}
	servers, ok := config[ompMCPServers].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	servers[relay.MCPServerName] = map[string]any{
		"command": lichBin,
		"args":    []string{relay.MCPSubcommand},
	}
	config[ompMCPServers] = servers

	body, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", path, err)
	}
	return append(body, '\n'), nil
}

// ompAgentDir resolves the directory omp keeps its state in — extensions, MCP
// servers and sessions all live under it. Three answers, in the precedence
// `omp config path` was measured to apply: a named profile moves the whole
// directory and wins outright, then the explicit override, then the default.
func ompAgentDir() (string, error) {
	if profile := os.Getenv("OMP_PROFILE"); profile != "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".omp", "profiles", profile, "agent"), nil
	}
	if dir := os.Getenv("PI_CODING_AGENT_DIR"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".omp", "agent"), nil
}

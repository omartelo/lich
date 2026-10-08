package terminal

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/store"
)

// The conversation lists of the five providers that describe a conversation in
// a metadata file or a database row rather than in the transcript's own head.

// kiroConversations reads ~/.kiro/sessions/cli/<id>.json, the interactive
// store, which is the one kiroSessionPath resumes from. Its
// session_created_reason is no sub-agent mark: 2.21.0 wrote "subagent" on every
// conversation measured, the ones a user opened by hand included.
func kiroConversations() []store.Conversation {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(home, ".kiro", "sessions", "cli", "*.json"))
	var out []store.Conversation
	for _, path := range paths {
		var meta struct {
			ID        string `json:"session_id"`
			Cwd       string `json:"cwd"`
			Title     string `json:"title"`
			UpdatedAt string `json:"updated_at"`
		}
		if !readJSON(path, &meta) || meta.ID == "" || meta.Cwd == "" {
			continue
		}
		out = append(out, store.Conversation{
			Kind: providers.Kiro, ID: meta.ID, Title: meta.Title, Cwd: meta.Cwd,
			UpdatedAt: rfc3339OrModTime(meta.UpdatedAt, path),
		})
	}
	return out
}

// cursorConversations reads chats/<md5(cwd)>/<id>/meta.json under Cursor's
// config directory, which names the cwd the md5 hides. A chat `create-chat`
// made and nobody talked in has no conversation to resume.
func cursorConversations() []store.Conversation {
	base, ok := cursorConfigDir()
	if !ok {
		return nil
	}
	paths, _ := filepath.Glob(filepath.Join(base, "chats", "*", "*", "meta.json"))
	var out []store.Conversation
	for _, path := range paths {
		var meta struct {
			Cwd             string `json:"cwd"`
			UpdatedAtMs     int64  `json:"updatedAtMs"`
			HasConversation bool   `json:"hasConversation"`
		}
		if !readJSON(path, &meta) || !meta.HasConversation || meta.Cwd == "" {
			continue
		}
		dir := filepath.Dir(path)
		out = append(out, store.Conversation{
			Kind: providers.Cursor, ID: filepath.Base(dir), Cwd: meta.Cwd,
			Title:     cursorTitle(filepath.Join(dir, "prompt_history.json")),
			UpdatedAt: meta.UpdatedAtMs / int64(time.Second/time.Millisecond),
		})
	}
	return out
}

// cursorTitle is the first prompt of a chat's history that is not a slash
// command. Cursor names every chat "New Agent", so its own name says nothing.
func cursorTitle(path string) string {
	var prompts []string
	if !readJSON(path, &prompts) {
		return ""
	}
	for _, p := range prompts {
		if !strings.HasPrefix(p, "/") {
			if title := promptTitle(p); title != "" {
				return title
			}
		}
	}
	return ""
}

// antigravityConversations reads the conversation index Antigravity keeps
// beside its conversations, keeping the rows whose conversation database still
// exists: that file is what antigravityConversationPath resumes from.
func antigravityConversations() []store.Conversation {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	index := filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
	return queryConversations(index,
		`SELECT conversation_id, title, preview, workspace_uris FROM conversation_summaries
		  WHERE parent_conversation_id = ''`,
		func(rows *sql.Rows) (store.Conversation, bool) {
			var id, title, preview, workspaces string
			if rows.Scan(&id, &title, &preview, &workspaces) != nil {
				return store.Conversation{}, false
			}
			db, ok := antigravityConversationPath(id)
			if !ok {
				return store.Conversation{}, false
			}
			c := store.Conversation{
				Kind: providers.Antigravity, ID: id, Cwd: firstWorkspace(workspaces),
				Title: firstNonEmpty(title, promptTitle(preview)), UpdatedAt: modTime(db),
			}
			return c, c.Cwd != ""
		})
}

// firstWorkspace is the directory of the first file:// URI in Antigravity's
// JSON list of workspaces.
func firstWorkspace(uris string) string {
	var list []string
	if json.Unmarshal([]byte(uris), &list) != nil || len(list) == 0 {
		return ""
	}
	u, err := url.Parse(list[0])
	if err != nil || u.Scheme != "file" {
		return ""
	}
	path := u.Path
	// A Windows URI keeps its drive behind the root: file:///C:/work is /C:/work.
	if len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.FromSlash(path)
}

// opencodeConversations reads the session table of opencode's one database. A
// sub-agent is a session with a parent; an archived one the user put away.
func opencodeConversations() []store.Conversation {
	path, ok := opencodeSessionDB()
	if !ok {
		return nil
	}
	return queryConversations(path,
		`SELECT id, directory, title, time_updated FROM session
		  WHERE parent_id IS NULL AND time_archived IS NULL`,
		func(rows *sql.Rows) (store.Conversation, bool) {
			c := store.Conversation{Kind: providers.OpenCode}
			var updatedMs int64
			if rows.Scan(&c.ID, &c.Cwd, &c.Title, &updatedMs) != nil {
				return store.Conversation{}, false
			}
			c.UpdatedAt = updatedMs / int64(time.Second/time.Millisecond)
			return c, c.Cwd != ""
		})
}

// crushConversations reads the database of every checkout Crush lists in its
// project index. The database read is the one crushSessionDB resumes from, in
// the checkout itself, whatever data_dir the index names.
func crushConversations() []store.Conversation {
	base, ok := harnessDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
	if !ok {
		return nil
	}
	var index struct {
		Projects []struct {
			Path string `json:"path"`
		} `json:"projects"`
	}
	if !readJSON(filepath.Join(base, "crush", "projects.json"), &index) {
		return nil
	}
	var out []store.Conversation
	for _, p := range index.Projects {
		db, ok := crushSessionDB(p.Path)
		if !ok {
			continue
		}
		out = append(out, queryConversations(db,
			`SELECT id, title, updated_at FROM sessions
			  WHERE parent_session_id IS NULL OR parent_session_id = ''`,
			func(rows *sql.Rows) (store.Conversation, bool) {
				c := store.Conversation{Kind: providers.Crush, Cwd: p.Path}
				if rows.Scan(&c.ID, &c.Title, &c.UpdatedAt) != nil {
					return store.Conversation{}, false
				}
				return c, true
			})...)
	}
	return out
}

// queryConversations runs one read-only query against a provider's database
// and keeps the rows scan accepts. A database that is missing, locked past the
// busy timeout or of another shape answers nothing.
func queryConversations(
	path, query string, scan func(*sql.Rows) (store.Conversation, bool),
) []store.Conversation {
	db, ok := openSessionDB(path)
	if !ok {
		return nil
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(query)
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()
	var out []store.Conversation
	for rows.Next() {
		if c, ok := scan(rows); ok {
			out = append(out, c)
		}
	}
	return out
}

func readJSON(path string, into any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return json.Unmarshal(data, into) == nil
}

func rfc3339OrModTime(stamp, path string) int64 {
	if t, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
		return t.Unix()
	}
	return modTime(path)
}

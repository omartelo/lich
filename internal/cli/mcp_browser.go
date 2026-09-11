package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/omartelo/lich/internal/browser"
)

// browserTools are this session's Chromium sidecar — the same window Browser
// tab opens. They are not in mcpTools because that file is already the relay
// surface; allMCPTools concatenates the two so tools/list cannot drift.
var browserTools = []mcpTool{
	{
		Name: "browser_open",
		Description: "Open a page in this lich session's browser. Creates the browser if " +
			"this session has none (headless unless the user already opened Browser tab on " +
			"this card). The URL must be http, https, or omitted (about:blank). file, " +
			"javascript and data URLs are refused. This is not the lich window.",
		Schema: schema(map[string]any{
			"url": property("string", "Page to load. Omit for about:blank."),
		}),
		Run: func(c *client, a mcpArgs) (string, error) {
			owner, err := c.browserOwner()
			if err != nil {
				return "", err
			}
			var h browser.Handle
			if err := c.call("browser.Open", []any{owner, a.text("url")}, openCall, &h); err != nil {
				return "", err
			}
			return encodeJSON(h)
		},
	},
	{
		Name: "browser_info",
		Description: "Read this session's current page: URL, title, visible text, and " +
			"interactive elements (index, tag, label, href, box). Form values are never " +
			"included — filled/checked say that typing landed. Call this before click/type " +
			"so indexes match. Read-only.",
		Schema:   schema(map[string]any{}),
		ReadOnly: true,
		Run:      mcpBrowserInfo,
	},
	{
		Name: "browser_click",
		Description: "Click in this session's browser. Prefer index from browser_info, or a " +
			"CSS selector, or viewport x and y. Sends a real mouse event, not element.click().",
		Schema: schema(map[string]any{
			"index":    property("integer", "Index from the last browser_info."),
			"selector": property("string", "CSS selector."),
			"x":        property("number", "Viewport X."),
			"y":        property("number", "Viewport Y."),
		}),
		Run: func(c *client, a mcpArgs) (string, error) {
			return mcpBrowserVoid(c, "browser.Click", []any{mcpTarget(a)}, shortCall)
		},
	},
	{
		Name: "browser_type",
		Description: "Type into this session's browser. Target as for browser_click; omit it " +
			"to type into whatever is focused. clear replaces the field first. The text never " +
			"comes back through browser_info.",
		Schema: schema(map[string]any{
			"text":     property("string", "Keys to send."),
			"clear":    property("boolean", "Replace the field before typing."),
			"index":    property("integer", "Index from the last browser_info."),
			"selector": property("string", "CSS selector."),
			"x":        property("number", "Viewport X."),
			"y":        property("number", "Viewport Y."),
		}, "text"),
		Run: func(c *client, a mcpArgs) (string, error) {
			return mcpBrowserVoid(c, "browser.Type", []any{a.text("text"), a.flag("clear"), mcpTarget(a)}, shortCall)
		},
	},
	{
		Name: "browser_press",
		Description: "Press one named key in this session's browser (Enter, Tab, Escape, " +
			"Backspace, Delete, Space, ArrowUp/Down/Left/Right, Home, End, PageUp, PageDown). " +
			"Real key event, not typed text — use browser_type for that. Optional target " +
			"focuses first, as for browser_type.",
		Schema: schema(map[string]any{
			"key":      property("string", "Named key: Enter, Tab, Escape, …"),
			"index":    property("integer", "Index from the last browser_info."),
			"selector": property("string", "CSS selector."),
			"x":        property("number", "Viewport X."),
			"y":        property("number", "Viewport Y."),
		}, "key"),
		Run: func(c *client, a mcpArgs) (string, error) {
			return mcpBrowserVoid(c, "browser.Press", []any{a.text("key"), mcpTarget(a)}, shortCall)
		},
	},
	{
		Name: "browser_screenshot",
		Description: "Capture this session's page to a PNG file and return the path. Do not " +
			"inline the bytes. Optional path; otherwise lich writes one next to the browser profile.",
		Schema: schema(map[string]any{
			"path": property("string", "Destination PNG. Omit to let lich choose."),
		}),
		ReadOnly: true,
		Run: func(c *client, a mcpArgs) (string, error) {
			owner, err := c.browserOwner()
			if err != nil {
				return "", err
			}
			var dest string
			if err := c.call("browser.Screenshot", []any{owner, a.text("path")}, openCall, &dest); err != nil {
				return "", err
			}
			return dest, nil
		},
	},
	{
		Name:        "browser_navigate",
		Description: "Load a URL in this session's already-open browser. Same allowlist as browser_open.",
		Schema: schema(map[string]any{
			"url": property("string", "http(s) URL, or about:blank."),
		}, "url"),
		Run: func(c *client, a mcpArgs) (string, error) {
			owner, err := c.browserOwner()
			if err != nil {
				return "", err
			}
			var h browser.Handle
			if err := c.call("browser.Navigate", []any{owner, a.text("url")}, openCall, &h); err != nil {
				return "", err
			}
			return encodeJSON(h)
		},
	},
	{
		Name:        "browser_reload",
		Description: "Reload this session's current page.",
		Schema:      schema(map[string]any{}),
		Run:         func(c *client, _ mcpArgs) (string, error) { return mcpBrowserVoid(c, "browser.Reload", nil, shortCall) },
	},
	{
		Name:        "browser_back",
		Description: "Go back in this session's browser history.",
		Schema:      schema(map[string]any{}),
		Run:         func(c *client, _ mcpArgs) (string, error) { return mcpBrowserVoid(c, "browser.Back", nil, shortCall) },
	},
	{
		Name:        "browser_forward",
		Description: "Go forward in this session's browser history.",
		Schema:      schema(map[string]any{}),
		Run: func(c *client, _ mcpArgs) (string, error) {
			return mcpBrowserVoid(c, "browser.Forward", nil, shortCall)
		},
	},
	{
		Name:        "browser_scroll",
		Description: "Scroll this session's page. dy is CSS pixels; positive is down. Default 400.",
		Schema: schema(map[string]any{
			"dy": property("integer", "Pixels to scroll. Default 400."),
		}),
		Run: func(c *client, a mcpArgs) (string, error) {
			dy := 400
			if n := a.optionalInt("dy"); n != nil {
				dy = *n
			}
			return mcpBrowserVoid(c, "browser.Scroll", []any{dy}, shortCall)
		},
	},
	{
		Name:        "browser_list",
		Description: "This session's browser, or an empty list. Read-only.",
		Schema:      schema(map[string]any{}),
		ReadOnly:    true,
		Run:         mcpBrowserList,
	},
	{
		Name: "browser_close",
		Description: "Close this session's browser. The lich window is untouched. Closing the " +
			"session card also closes it.",
		Schema: schema(map[string]any{}),
		Run:    func(c *client, _ mcpArgs) (string, error) { return mcpBrowserVoid(c, "browser.Close", nil, shortCall) },
	},
}

func (c *client) browserOwner() (string, error) {
	id := c.sessionID()
	if id == "" {
		return "", fmt.Errorf("browser tools run inside a lich session")
	}
	return id, nil
}

func mcpTarget(a mcpArgs) browser.Target {
	t := browser.Target{Selector: a.text("selector")}
	t.Index = a.optionalInt("index")
	t.X = a.optionalFloat("x")
	t.Y = a.optionalFloat("y")
	return t
}

func mcpBrowserVoid(c *client, method string, extra []any, wait time.Duration) (string, error) {
	owner, err := c.browserOwner()
	if err != nil {
		return "", err
	}
	args := append([]any{owner}, extra...)
	if err := c.call(method, args, wait, nil); err != nil {
		return "", err
	}
	return "ok", nil
}

func encodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func mcpBrowserInfo(c *client, _ mcpArgs) (string, error) {
	owner, err := c.browserOwner()
	if err != nil {
		return "", err
	}
	var info browser.PageInfo
	if err := c.call("browser.Info", []any{owner}, openCall, &info); err != nil {
		return "", err
	}
	return encodeJSON(info)
}

func mcpBrowserList(c *client, _ mcpArgs) (string, error) {
	owner, err := c.browserOwner()
	if err != nil {
		return "", err
	}
	var list []browser.Handle
	if err := c.call("browser.List", []any{owner}, shortCall, &list); err != nil {
		return "", err
	}
	if list == nil {
		list = []browser.Handle{}
	}
	return encodeJSON(list)
}

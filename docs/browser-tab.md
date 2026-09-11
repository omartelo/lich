# Decision: the session browser is a sidecar window, not an in-app tab

**Status: shipped.** Right-click a session card → **Browser tab** opens a
Chromium window owned by that session. The agent drives the same page through
`lich browser` and the `browser_*` MCP tools.

## Why not a tab inside lich

lich's window is a system Chromium in `--app` mode (`docs/chromium-shell.md`).
lich does not embed Chromium: no `WebContentsView`, no `<webview>`, no CEF.
An iframe of arbitrary `https://` is unsafe (X-Frame-Options, and the RPC
token in the page URL). Attaching CDP to the `--app` process would expose the
UI and the session token.

So the product is a **second Chromium process** on `chromium.FindBrowser`
(system Chrome/Edge/Chromium/… — never `lich-shell`), with a **different**
`user-data-dir` than `lich/chromium-profile`. Closing that window does not
quit lich. Closing the session card kills the sidecar.

## One browser per card

The human entry point is the session's context menu (after Pin, before Open
Terminal), not the file tree and not a dock tab. Clicking **Browser tab**:

- none yet → headed window at `about:blank`, chromedp attached
- already headed → focus that window
- headless only (the agent opened first) → replace with a headed window at
  the same URL (chromedp cannot promote a headless process in place)

The agent without the menu uses `browser_open`: headless if nobody asked for a
window, reuse the headed one if the human already did. MCP and `lich browser`
target `LICH_SESSION_ID`, not a pool of ids.

## What the agent may load

`http`, `https`, `about:blank`. Not `file:`, `javascript:`, `data:`, `chrome:`.
Page-info never serializes form control values (passwords, recovery phrases);
agents get `filled` / `checked`. Screenshots are files, not base64 in the
transcript. Click, type and press are real input events, not `element.click()`
or `el.value = …`. `press` is named keys only (Enter, Tab, Escape, arrows, …);
text still goes through `type`.

## Providers

Claude Code and Codex see the tools at spawn (`lich mcp`). Crush and oh-my-pi
see them once the plugin has registered that server. **opencode does not**:
its plugin cannot register MCP and still defines the original seven tools —
see `docs/ceilings.md`. `lich browser` still works in that PTY.

`evaluate` (arbitrary JS in the page) is out of v1.

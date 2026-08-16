# lich

`lich` is a **harness for AI coding agents**: a Go backend serving an embedded React frontend to a system
Chromium window in `--app` mode — no Electron, no webview toolkit (`docs/chromium-shell.md`). It is built for
other developers to use, not only its author: docs, errors and defaults answer to a stranger. Linux first;
Windows and macOS are experimental, and there is no hardware here for either.

**This file is instruction, not documentation.** Rules, gates and the workflow live here. What the project *is*
lives in the code, `docs/` and `CHANGELOG.md` — never restate any of it here.

## Where things are written down

- **Deliberate limits and the traps they set**: `docs/ceilings.md`. Read it before changing anything a session
  touches — spawn and naming, hooks, transcripts and cost, git and worktree flows, persistence, plugin install.
  A change that sets a new trap or closes an old one moves that file in the same PR.
- **Frontend rules and the visual system**: `frontend/CLAUDE.md` and `frontend/DESIGN.md`.
- **Session hooks**: `docs/hooks/` is the canonical, contract-first spec. lich owns the server side; the scripts
  live in the companion repo `omartelo/lich-plugin`, so an endpoint or payload change breaks a repo this one
  cannot see — move the contract first, then both sides.
- **Every task**: `task --list`. `task dev` gets its own DB, port and Chromium profile; it never touches an
  installed lich's workspace.
- **User-facing feature history**: `CHANGELOG.md`.

## Rules of the codebase

- Go 1.26, pure Go: `CGO_ENABLED=0` and a fully static binary are a constraint, not a default.
- OS-specific code is selected by build tags behind small seams, never by runtime checks — the PTY is the model
  (`internal/terminal`).
- Service shapes are hand-owned in `frontend/src/lib/api-types.ts`: touch a Go struct's JSON tags and that mirror
  moves in the same change. There is no codegen.

## Local Gate (before every commit / PR)

- `gofmt -l .` clean (fix with `gofmt -w .`) and `go vet ./...` clean.
- `cd frontend && pnpm check` clean — biome is the frontend's gofmt + vet (fix with `pnpm format`).
- `go test ./...` (backend) and `cd frontend && pnpm test` (frontend) green — or `task test` for both.
- `cd frontend && pnpm build` succeeds (tsc typecheck + vite).
- Shipped anything a user can see? Its `CHANGELOG.md` `[Unreleased]` entry lands in the same PR — the release notes
  are read from there, so an entry written later is an entry that missed its release.
- Touched an OS seam or a `_test.go` build tag? Run the same cross-compile loop CI runs:
  `for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done`
  Cross-compiling only proves it builds; the backend suite runs on real Windows and macOS runners on every PR,
  and that is the answer to trust.

## Hard Invariants

Non-negotiable rules. A violation means the work is not done.

1. **Test coverage ≥ 80%**, backend and frontend. CI measures and reports the number but does not auto-fail below
   the bar — it is held in review, so read the summary. OS/framework boundaries (the PTY, the Chromium
   launcher/zenity subprocesses, WebSocket wiring, the `main` bootstrap, xterm.js internals) are the documented
   exception: cover the pure logic and leave the boundary itself alone.
2. **Tests answer to the contract, never the other way round.** Never weaken, skip, delete or rewrite a test to buy
   a green run or a coverage number. A test may change for exactly two reasons: the contract changed, or the test
   asserted something the contract never promised — name which one in the diff. When in doubt, change the product,
   not the test.
3. **A flake is a bug, and it has a root cause.** Never re-run CI until it goes green, never dismiss a failure as
   "unrelated" without proving it. Reproduce (`go test -count=200 -run TestX ./pkg/`), fix the cause, and measure
   the failure rate before and after.
4. **Clean code.** Small focused functions (< 50 lines); cohesive files (200–400 lines typical, 800 max); no
   nesting deeper than 4 levels; comments only for the *why*; errors handled explicitly, never swallowed; no magic
   values; no secrets in source.
5. **A session feature is traced across every provider.** `internal/providers.Registry` is the checklist —
   Claude Code, Codex, opencode, oh-my-pi, Crush. Anything a session touches (spawn flags, hooks, resume,
   transcripts, plugin install, MCP) is designed against all five, not just the provider at hand. Equal behaviour
   is not always possible — but the gap must be deliberate and written down in the same PR: a `docs/ceilings.md`
   bullet naming which providers are out and why. A feature that silently works on a single provider is not done.

## Releases

The version comes from the git tag (`git describe` in the Taskfile, env `VERSION` overrides) and is injected into
`build/linux/nfpm/nfpm.yaml` — never hand-edit it there. Before tagging:

- [ ] Local gate green, backend with `-race`.
- [ ] Move `CHANGELOG.md`'s `[Unreleased]` entries under a new `vX.Y.Z` heading and refresh the compare links.
- [ ] Push the `vX.Y.Z` tag — `.github/workflows/release.yml` does the rest, and reads the notes from that section.

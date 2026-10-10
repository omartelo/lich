---
name: lich-build-dev
description: Build, run and verify lich from an agent session without taking down the user's own lich window. Use before building anything (frontend, Go binary, the CEF window), before starting or stopping `task dev`, before running tests or the local gate, and before verifying a UI change in a running lich.
---

# lich build, dev rig and local gate

What the gate *is* lives in the root `CLAUDE.md` › Local Gate; the frontend commands in `frontend/CLAUDE.md` ›
Commands; every task in `task --list`; the window in `docs/chromium-shell.md`; first-build prerequisites in
`CONTRIBUTING.md`. This skill is the procedure for running them from inside a lich session, and the traps that
procedure avoids.

**You are running inside the user's live lich window.** Every session on their screen, this one included, is
a child of it. Never `pkill`/`killall`/`kill` by pattern (`chromium`, `chromium-profile`, `lich`, `lich-shell`,
`cef`, `vite`, `node`): the pattern matches the user's window, and its death takes every session down. Kill
only a PID or process group you started and wrote down.

## 1. First steps in a fresh worktree

```bash
W=$(git rev-parse --show-toplevel)       # use absolute paths from here on
which -a go                              # only mise paths (~/.local/share/mise/...)
(cd "$W/frontend" && pnpm install --frozen-lockfile)
```

A new worktree has no `frontend/node_modules` and no `frontend/dist`. `go vet` and `go build` need
`frontend/dist` (`main.go` embeds it), so build the frontend before any Go step (the gate below does).

## 2. The local gate, in order

One step at a time, each inside the memory jail from `CLAUDE.md` › Local Gate (`cap` below is that command):
gate steps run in parallel have killed the user's window with SIGBUS. Never start two of these at once, nor a
gate next to a `task dev` build. Keep `$T` short: `go test` hands it to the tests as their TMPDIR, and the
sandbox suite's Unix sockets break past the 108-byte path limit.

```bash
W=$(git rev-parse --show-toplevel); T=/var/tmp/lich-gate; mkdir -p "$T"
cap() { systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=2G --nice=10 \
  env GOTMPDIR="$T" TMPDIR="$T" GOFLAGS=-p=2 "$@"; }
cd "$W/frontend" && pwd &&
  ./node_modules/.bin/biome ci . &&
  cap ./node_modules/.bin/tsc &&
  cap ./node_modules/.bin/vite build --mode production &&
  cap ./node_modules/.bin/vitest run --coverage &&
cd "$W" && pwd &&
  test -z "$(gofmt -l .)" &&
  cap go vet ./... &&
  cap go test ./... &&
  echo GATE GREEN
```

- Touched `shell/`: add `cd "$W/shell" && cargo fmt --check && cap cargo clippy --release --all-targets -- -D
  warnings && cap cargo test --release`.
- Touched an OS seam or a `_test.go` build tag: the cross-compile loop in `CLAUDE.md`, under `cap`.
- Prose-only change and no frontend build? `go vet` still needs `frontend/dist`: `mkdir -p frontend/dist &&
  touch frontend/dist/.keep`, and remove it after.

## 3. `task dev` for UI checks

`task dev` is isolated from the user's lich: its own DB, backend port 47822, Chromium profile and window class
`lichdev`. It depends on `build:shell`: the CEF distribution comes from the per-user cache shared by every
worktree (`bash build/cef.sh --dir` prints it), but `shell/target` (the Rust build and the fetch tool) is per
worktree, so the first run in a fresh worktree compiles the window (about 40 s on this machine) plus
`pnpm install` and a frontend build. Say so before starting it.

```bash
ss -ltnp 'sport = :47822 or sport = :9245'     # empty, or someone else's task dev is up
T=/var/tmp/lich-gate; mkdir -p "$T"
setsid task dev > "$T/dev.log" 2>&1 < /dev/null & echo $! > "$T/dev.pid"
# ready when: curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:47822/ prints 200
kill -TERM -- -"$(cat "$T/dev.pid")"           # stop: your own process group, Vite included
```

- Only one `task dev` per machine: the backend port 47822 and the dev DB are fixed. Port 9245 taken with
  47822 free is an orphaned Vite from a `task dev` whose window was closed; it is not yours, so ask before
  stopping it, or run with `LICH_VITE_PORT=9246`.
- A rebase or checkout under a running `task dev` can leave Vite serving the old modules; only a restart fixes
  it. Ask before restarting a `task dev` you did not start.

## 4. Traps

- **rtk rewrites commands and fakes exit codes.** A bare `pnpm exec biome ci .` becomes `rtk lint ci .`, prints
  "No issues found" and exits 1. Run the binaries in `frontend/node_modules/.bin` directly, as above, or
  prefix `rtk proxy`. Output that looks cut is the same hook: `rtk proxy <cmd>`.
- **Read exit codes, not tails.** `| tail` hides the exit status; redirect to a log under `$T` and echo `$?`.
- **cwd drifts.** A failed `cd frontend && ...` leaves the shell in another directory, sometimes another
  checkout. Use `$W` and start a step with `pwd &&` when it matters.
- **vitest can serve stale test files.** After adding tests, check that "Tests N passed" grew by what you
  added. If not, `rm -rf frontend/node_modules/.vite/vitest` and run again.
- **Two Go toolchains.** mise exports `GOROOT`; a stray `/usr/bin/go` obeys it and fails intermittently with
  stdlib mismatches. `which -a go` must list only mise paths.

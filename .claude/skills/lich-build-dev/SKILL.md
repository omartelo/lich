---
name: lich-build-dev
description: Build, run and verify lich from an agent session without taking down the user's own lich window. Use before building anything (frontend, Go binary, the CEF window), before starting or stopping `task dev` or running the native window (`lich native`, `native/`), before running tests or the local gate, and before verifying a UI change in a running lich.
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

## 4. The native window (`lich native`) on its own backend

`native/` is a separate Go module and binary (`docs/native-window.md`). Never point it at the user's lich: the
backend's `/events` and `/ws` serve one client each, so a native window on the user's backend takes the sockets
from their real window. `native/slice.sh` runs a third, isolated backend instead: its own config dir and DB
(`~/.cache/lich-native-slice/config`), port 47901, no Chromium, separate from both the user's lich and
`task dev`. The backend outlives the window, so its sessions survive between runs.

```bash
W=$(git rev-parse --show-toplevel); T=/var/tmp/lich-gate; mkdir -p "$T"
cap() { systemd-run --user --scope --quiet -p MemoryMax=6G -p MemorySwapMax=2G --nice=10 \
  env GOTMPDIR="$T" TMPDIR="$T" GOFLAGS=-p=2 "$@"; }
ss -ltnp 'sport = :47901'                       # empty, or a slice backend is already up (see below)
# libghostty-vt: built once per machine at native/GHOSTTY_COMMIT; slice.sh's header says how
export PKG_CONFIG_PATH=$(dirname "$(find ~/.cache/lich-native-slice/libghostty-vt -name '*.pc' | head -1)")
# this checkout's backend (needs frontend/dist, section 1) and window, built in the jail
cd "$W" && cap go build -o "$T/lich" . &&
cd "$W/native" && cap go build -o cmd/lich-native/lich-native ./cmd/lich-native &&
LICH_BIN="$T/lich" setsid "$W/native/slice.sh" "$W" > "$T/native.log" 2>&1 < /dev/null & echo $! > "$T/native.pid"
kill "$(cat "$T/native.pid")"                   # close the window: your PID only
"$W/native/slice.sh" stop                       # stop the slice backend and its sessions
```

- The argument opens that checkout as the project on the backend's first run; later runs reuse its projects.
- A slice backend already on 47901 is reused as is, whatever binary it runs. Backend code changed, or it is not
  yours? Ask before `slice.sh stop`: it may hold the user's review window and sessions.
- Without `LICH_BIN` the backend is the installed `lich` on PATH, enough when only `native/` changed.
- Drawing changed: `timeout 15 cmd/lich-native/lich-native -runtime
  ~/.cache/lich-native-slice/config/lich/runtime-dev.json -shot out.png -shot-after 4s` writes a frame; the
  window stays open after it, hence the `timeout`. It takes the backend's sockets for those seconds, so not
  while the user has a native window on the same backend. Look at the PNG; the user's review in a real window
  is the one to trust.

## 5. Traps

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

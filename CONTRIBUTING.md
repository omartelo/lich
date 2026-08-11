# Contributing

Bugs and ideas are welcome, and so are patches. This file is the short version;
`CLAUDE.md` holds the invariants and the deliberate limits behind them.

## Reporting

- **Something is broken** → the [bug form](https://github.com/omartelo/lich/issues/new?template=bug.yml).
  Settings → Help → *Report a bug* opens it with the version and platform
  already filled in, and *Open log folder* puts you next to the file to attach.
  Read the log before attaching it: it carries paths, project and branch names
  and your gh login — never a session token.
- **Something is missing** → the [feature form](https://github.com/omartelo/lich/issues/new?template=feature.yml).
  Describe the problem first; the shape you have in mind is optional, and a
  well-described problem often has a better answer than either of us reaches
  for first.
- **A security problem** → not an issue. See [SECURITY.md](SECURITY.md).
- **The session hooks** (titles, git refresh, resume) ship from
  [lich-plugin](https://github.com/omartelo/lich-plugin) — report those there.

## Getting it running

Prerequisites: **Go 1.25+**, **Node + pnpm**, and **[Task](https://taskfile.dev)**.
No C toolchain and no system dev libraries — the backend is pure Go
(`CGO_ENABLED=0`). At runtime you need a Chromium-family browser on `PATH`,
plus `zenity` on Linux for the folder picker.

```bash
task dev      # Vite HMR + backend
task --list   # everything else
```

`task dev` gets its own database, port and Chromium profile, so it never
touches the workspace of a lich you have installed.

The frontend is **pnpm**, not npm — `npm install` errors out.

## Before you open a pull request

Run the same gate CI runs:

```bash
gofmt -l .                      # must print nothing (fix: gofmt -w .)
go vet ./...
cd frontend && pnpm check       # biome — the frontend's gofmt + vet
cd frontend && pnpm build       # tsc typecheck + vite
task test                       # both suites
```

Then:

- **Shipping something a user can see?** Add its `CHANGELOG.md` `[Unreleased]`
  entry in the same pull request. Release notes are read from that file, so an
  entry written later is an entry that missed its release.
- **Touched OS-specific code or a `_test.go` build tag?** Run the cross-compile
  loop — CI runs the backend suite on real Windows and macOS runners for every
  pull request, so that is where the answer comes from; this only saves you a
  round trip:
  ```bash
  for os in linux darwin windows; do GOOS=$os go build ./... && GOOS=$os go vet ./...; done
  ```
- **Touched a Go struct's JSON tags?** `frontend/src/lib/api-types.ts` mirrors
  them by hand — there is no codegen, so it moves in the same change.
- **Touched the UI?** `frontend/DESIGN.md` is the visual system, and it is
  opinionated: surfaces are defined by space, hover and hairline seams rather
  than by borders and nested boxes.

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org)
(`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`, `perf:`, `ci:`).
`main` takes squash and rebase merges only.

## Two rules worth stating outright

**Tests answer to the contract, never the other way round.** Never weaken, skip
or delete a test to buy a green run. A test changes for exactly two reasons:
the contract changed, or it asserted something the contract never promised —
say which one in the pull request.

**A flake is a bug with a root cause.** Don't re-run CI until it goes green.
Reproduce it (`go test -count=200 -run TestX ./pkg/`), fix the cause, and say
what the failure rate was before and after.

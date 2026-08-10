# Hook contract fixtures

The contracts in the parent directory are prose; these are the same contracts as
**bytes**. One file per contract, one case per line, each naming a payload and
what lich does with it.

They exist because the two sides of every hook live in different repositories:
lich owns the server side, the plugin ([`omartelo/lich-plugin`](https://github.com/omartelo/lich-plugin))
owns the scripts that build the payloads. A field renamed on one side used to
break the other silently — nothing in either repo's tests would go red. Both
sides asserting against the same lines turns that into a failing test on
whichever side moved first.

## Files

| Fixture                | Contract                                     | Endpoint           |
|------------------------|----------------------------------------------|--------------------|
| `session-state.jsonl`  | [session-state.md](../session-state.md)      | `/hook`            |
| `session-start.jsonl`  | [session-start.md](../session-start.md)      | `/session-start`   |
| `session-title.jsonl`  | [session-title.md](../session-title.md)      | `/session-title`   |
| `session-touched.jsonl`| [session-touched.md](../session-touched.md)  | `/session-touched` |

The file is named after the contract, not the endpoint — `/hook` predates the
naming and keeps its path for the plugin releases already pointing at it.

## Case format

One JSON object per line:

| Field    | Meaning                                                              |
|----------|----------------------------------------------------------------------|
| `name`   | What the case is. Unique within its file; used as the subtest name.  |
| `body`   | The request body, as JSON. Exactly what a client POSTs.              |
| `raw`    | The request body as a **string**, for bodies that are not valid JSON. |
| `accept` | lich accepts it (`204`). The field values it holds *after* parsing.   |
| `reject` | lich rejects it (`400`). Why, in prose.                              |

Every line carries exactly one of `body` / `raw`, and exactly one of `accept` /
`reject`.

`accept` is the payload **normalized**, not echoed: it is what the contract's
defaulting and trimming produce, so it is where `provider` defaults to `claude`
and a title loses its surrounding whitespace. It lists only the fields a case is
about — a field absent from `accept` is not asserted, which is how the
deprecated `claude_session_id` stays out of the expectations while still being
exercised by the payloads that send it.

`reject` prose is documentation, not a matcher. lich's error strings are its own
and change freely; what the fixture fixes is *that* the payload is refused.

## Using them

- **lich (server side)** — `internal/terminal/fixtures_test.go` runs every case
  twice: through the contract's parse function, and as a real POST to a live
  transport, asserting the status code and the normalized fields. A contract
  without a fixture file fails that test, so a new hook cannot ship without one.
- **The plugin (client side)** — the payload a script builds must match an
  `accept` case's `body` for the shape it sends. Vendor the files or read them
  from this repo at
  `https://raw.githubusercontent.com/omartelo/lich/main/docs/hooks/fixtures/<name>.jsonl`.

## Changing them

A fixture moves for the same two reasons a test does: the contract changed, or
the fixture asserted something the contract never promised. Name which one in
the diff.

Adding a case is not a contract change — cover a payload the contract already
describes and ship it alone. Editing or deleting a case is: it means the
contract moved, so the prose above moves first, then this file, then lich's
server side, then the plugin. Never loosen a `reject` into an `accept` to get a
green run.

---
name: lich-i18n
description: Map and carry out translation work in lich, for both the interface language (React catalogs) and the prompt language (text lich and lich-plugin inject into agent sessions). Use when adding or changing any user-visible text or any text typed into a session, when adding a new language, when a parity or locale test fails, or when asked what a translation change touches.
---

# lich i18n

lich has two languages that never derive from each other:

- **Interface language**: lich's own screens. Catalogs in `frontend/src/lib/i18n/`, chosen in
  Settings › Appearance, stored as a localStorage pref.
- **Prompt language**: every word lich hands an agent (spawn briefing, `[lich]` relay notes, nudges,
  merge notices, MCP and CLI results, prompts the frontend types into a PTY, lich-plugin's mod text).
  Stored as the global `prompt.language` setting, exported to every session as `LICH_PROMPT_LANG`.

The rules for writing a message (keys, one sentence per key, plurals, `<Trans>`, what is never translated)
are in `frontend/CLAUDE.md` › Translations. This skill is the map of *where* a change lands. Start by
deciding which job this is.

## Job 1: a feature adds or changes text

Classify every string first. Who reads it?

| Reader | Where it goes | Language |
|---|---|---|
| The user, on a lich screen | `locales/<lang>/<namespace>.ts`, read with `t()` / `useT()` | interface |
| An agent, composed in Go (relay, briefing, notices, MCP/CLI output) | a field of `prompt.Catalog` (`internal/prompt/catalog.go`) | prompt |
| An agent, typed by the frontend into a PTY | `locales/<lang>/prompts.ts`, read with `prompt()` from `lib/i18n/prompt.ts` | prompt |
| An agent, added by a lich-plugin mod | `hooks/prompt-text.js` in `lichdotdev/lich-plugin` | prompt |
| Nobody localizes it | error text through `errorText()`, CHANGELOG, MCP tool descriptions, CLI `--help`, identifiers | none |

Then, in the same PR:

1. **English is the source.** Add the key to `locales/en/*` (or the `en.go` field) with the exact text.
2. **Every locale gets it.** One entry per file in `locales/*/` and per `internal/prompt/<lang>.go`.
   tsc fails on a missing TS key; `TestCatalogParity` fails on an empty Go field or mismatched verbs.
   Translate from English, never from another translation.
3. **Literal in every locale:** `[lich]`, tool and CLI names (`send_to_session`, `wait_for_answer`,
   `reply_to_session`, `open_session`, `close_session`, `list_worktrees`, `"$LICH_BIN" reply|wait`,
   `lich send`, `gh pr create`), ticket ids, status enums (`pending`, `answered`, `unread`, ...),
   parameter names, product names, key names, `{placeholders}` and Go format verbs.
4. **Text lich matches by equality** (`resumePrompt` is the precedent: persisted, then compared) must
   match the text of *any* locale: `slices.ContainsFunc(prompt.Langs, ...)`. A row written under one
   language is read back under another.
5. **Out-of-process text** (`internal/cli`, plugin hooks) reads `LICH_PROMPT_LANG`, fixed at spawn. If
   the new text only reaches sessions born after a change, that is the existing ceiling; don't add a
   second mechanism.
6. **Settings rows** with a translated title are written `title={t("key")}` and indexed by `titleKey`
   in `lib/settings-index.ts`.
7. **Tests stay in English.** Assert another locale with `setLocale(...)` / a `prompt.Langs` loop, never
   by editing an English assertion.
8. **Provider symmetry.** Prompt text reaches all eight providers through the relay. The spawn briefing
   only exists where `docs/ceilings.md` says it does; a gap that grows goes there and in both README
   provider tables.

## Job 2: add a language

Pick the BCP 47 tag first (`es`, `zh-CN`, `pt-BR`); the same string is the TS locale, the Go `Lang`, the
setting value, `LICH_PROMPT_LANG` and the plugin catalog key. Then touch every point below. Append the new
language at the **end** of every list so parallel additions merge as a union.

**Frontend** (`frontend/src/lib/i18n/`)
- `i18n.ts`: `LOCALES`, `LOCALE_NAMES` (the language named in itself: `Español`, `简体中文`), `CATALOGS`.
  `matchLocale` needs no change unless the language's script or region rules differ from "exact tag,
  then language prefix".
- `locales/<tag-lowercase>/`: one file per namespace in `locales/en/` (list it, don't remember it), each
  `satisfies Shape<typeof en>`, plus `index.ts` exporting the locale object `satisfies Shape<Messages>`.
- Plurals: write the CLDR forms the language uses (`PluralForms` allows `zero/one/two/few/many/other`);
  `Intl.PluralRules` picks. A language without plurals fills `other` (and `one` where en has it).
- Tests: the locale map in `i18n.test.ts`; a render test beside the others (`Trans.test.tsx`,
  `prompt.test.ts`, `lib-messages.test.ts`).

**Backend** (`internal/prompt/`)
- `prompt.go`: a `Lang` const, `Langs` (append), the `catalogs` map.
- `<tag>.go`: a full `Catalog`, including its `isOne` rule. `Plural` has only `One`/`Other`: a language
  with more CLDR forms (Russian, Arabic, Polish) extends `Plural` and `Pick` in the same PR.
- The spawn briefing must stay cmd.exe-safe (no `"` or `<>`); `compose_lang_test.go` checks every
  `Langs` entry, so a new language is covered once it is in `Langs`.
- Tests that loop `prompt.Langs` cover it automatically; add a language-specific case where the pt-BR
  or es one exists (`prompt_test.go`, `internal/cli/text_lang_test.go`, `internal/drop/notice_lang_test.go`,
  `internal/store/prompt_language_test.go`, `internal/terminal/prompt_language*_test.go`).

**Docs**
- `docs/hooks/README.md`: the accepted `LICH_PROMPT_LANG` values.
- `CHANGELOG.md` `[Unreleased]` › Added.
- README / README.zh-CN only if they enumerate languages.

**Companion repo** `lichdotdev/lich-plugin` (separate PR, after lich's)
- `hooks/prompt-text.js`: a catalog object registered under the exact tag.
- `tests/prompt-text.test.mjs`: parity, literals, one render per hook for the new tag.
- The mod engine never follows `$` across an import: hooks read `$.env.get("LICH_PROMPT_LANG")` at the
  call site and pass the string to `promptLang(tag)`. Run **both** `node --test tests/*.test.mjs` and
  `claude plugin test .`; only the second loads the mods the way Claude Code does.
- The plugin text reaches users only after a plugin release.

## Splitting the work across agents

A new language is one agent per repo side, not one per namespace: the catalogs are read best as a whole,
and the registration points are few. Converting untranslated code is the opposite: one agent per folder,
since folders don't overlap. Agents working in parallel conflict only on the registration lists
(`locales/*/index.ts`, `LOCALES`, `Langs`), and those resolve as a union.

## Done means

- The Local Gate in the root `CLAUDE.md` is green.
- No existing test assertion changed.
- `go test ./internal/prompt/` and `vitest run src/lib/i18n` pass.
- The new language was looked at in `task dev`: both selects, a session spawned after the switch, and
  one relay note.

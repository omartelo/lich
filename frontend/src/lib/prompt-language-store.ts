import { type Locale, matchLocale } from "@/lib/i18n/i18n"
import { Store } from "@/lib/rpc"

// The language of the text lich types into agent sessions. Unlike the
// interface language it lives in the backend settings table, because the
// backend is what composes that text (internal/prompt): the relay reads it at
// every message, a spawn reads it for the briefing and LICH_PROMPT_LANG.
// This store is the page's copy, loaded once and written through.
//
// The tags are the interface's own (en, pt-BR, zh-CN): both sides ship the same set.

// SETTING_KEY and GLOBAL_SCOPE mirror the Go side's promptLanguageKey and its
// global (project-less) scope.
const SETTING_KEY = "prompt.language"
const GLOBAL_SCOPE = ""

let language: Locale = "en"
let loaded = false
let revision = 0
const listeners = new Set<() => void>()

const notify = () => {
  for (const listener of listeners) listener()
}

// An absent value reads as English, as the backend reads it. A failed read
// keeps English too, the same answer the backend gives for one; the select
// still writes.
const load = async () => {
  const startedAt = revision
  try {
    const next = matchLocale(await Store.GetSetting(SETTING_KEY, GLOBAL_SCOPE))
    if (startedAt === revision && next !== language) {
      language = next
      notify()
    }
  } catch {
    // Keep the default, as cost-readout-store does.
  }
}

const loadOnce = () => {
  if (loaded) return
  loaded = true
  void load()
}

export const promptLanguageStore = {
  // A read starts the load too: the prompt composers read this without
  // subscribing, and the first text typed must not wait on the Settings pane.
  get: (): Locale => {
    loadOnce()
    return language
  },
  subscribe(listener: () => void): () => void {
    listeners.add(listener)
    loadOnce()
    return () => {
      listeners.delete(listener)
    }
  },
}

/** Writes the choice through; the page shows it at once and puts the old one
 * back if the backend refuses it. */
export function setPromptLanguage(next: Locale): Promise<void> {
  const previous = language
  const writeRevision = ++revision
  if (next !== language) {
    language = next
    notify()
  }
  return Store.SetSetting(SETTING_KEY, GLOBAL_SCOPE, next).then(
    () => {},
    (error: unknown) => {
      if (revision === writeRevision && language !== previous) {
        language = previous
        notify()
      }
      throw error
    },
  )
}

/** Drops the module state between tests. */
export function resetPromptLanguageStore(): void {
  language = "en"
  loaded = false
  revision++
  listeners.clear()
}

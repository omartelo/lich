import { useSyncExternalStore } from "react"
import { readPref, writePref } from "@/lib/prefs"
import type { At, ParamsArg, PluralForms, Shape } from "./catalog"
import { en, type MessageKey, type Messages } from "./locales/en"
import { ptBR } from "./locales/pt-br"

// The interface language. It is a page preference, not a workspace setting:
// it changes nothing the backend does, so it lives in localStorage like the
// theme. The language of the text lich types into agent sessions is a separate
// setting on purpose (prompt-language-store.ts, internal/prompt): people often
// run the interface in one language and talk to their agents in another.

export const LOCALES = ["en", "pt-BR"] as const
export type Locale = (typeof LOCALES)[number]

/** Each language named in itself, which is how someone who cannot read the
 * current interface still finds their own. */
export const LOCALE_NAMES: Record<Locale, string> = { en: "English", "pt-BR": "Português (Brasil)" }

const CATALOGS: Record<Locale, Shape<Messages>> = { en, "pt-BR": ptBR }

export const UI_LANGUAGE_PREF = "lich.appearance.language"

/** The locale for a BCP 47 tag: an exact match, then a language match (any
 * Portuguese reads pt-BR), then English. */
export function matchLocale(tag: string | null | undefined): Locale {
  if (!tag) return "en"
  const exact = LOCALES.find((locale) => locale.toLowerCase() === tag.toLowerCase())
  if (exact) return exact
  const language = tag.split("-")[0].toLowerCase()
  return LOCALES.find((locale) => locale.split("-")[0].toLowerCase() === language) ?? "en"
}

// The node test environment has neither storage nor a browser language; those
// suites run in English, which is what every existing assertion is written in.
function initialLocale(): Locale {
  if (typeof localStorage !== "undefined") {
    const stored = readPref(UI_LANGUAGE_PREF)
    if (stored !== null) return matchLocale(stored)
  }
  return matchLocale(typeof navigator === "undefined" ? null : navigator.language)
}

let current: Locale | null = null
const listeners = new Set<() => void>()

export function getLocale(): Locale {
  current ??= initialLocale()
  return current
}

export function setLocale(locale: Locale): void {
  writePref(UI_LANGUAGE_PREF, locale)
  if (locale === current) return
  current = locale
  for (const listener of listeners) listener()
}

export function subscribeLocale(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** The current locale, re-rendering the caller when it changes. */
export function useLocale(): Locale {
  return useSyncExternalStore(subscribeLocale, getLocale, getLocale)
}

/** t, for a component: the same function, but the component re-renders when
 * the language changes. Plain modules import t directly. */
export function useT(): typeof t {
  useLocale()
  return t
}

function lookup(locale: Locale, key: string): string | PluralForms {
  let node: unknown = CATALOGS[locale]
  for (const part of key.split(".")) {
    node = (node as Record<string, unknown>)[part]
  }
  if (node === undefined) {
    throw new Error(`i18n: no message "${key}" in ${locale}`)
  }
  return node as string | PluralForms
}

/** The template a key resolves to for the current locale, the plural form
 * already chosen by params.count. */
export function template(key: string, count: number | undefined): string {
  const locale = getLocale()
  const message = lookup(locale, key)
  if (typeof message === "string") return message
  if (count === undefined) {
    throw new Error(`i18n: "${key}" is a plural and needs a count`)
  }
  const form = new Intl.PluralRules(locale).select(count)
  return message[form] ?? message.other
}

const PLACEHOLDER = /\{(\w+)\}/g

/** The template split around its placeholders, each one swapped for its
 * value: what both t() and <Trans> render from. */
export function interpolate<V>(text: string, params: Record<string, V>): (string | V)[] {
  const parts: (string | V)[] = []
  let last = 0
  for (const match of text.matchAll(PLACEHOLDER)) {
    const name = match[1]
    if (!(name in params)) {
      throw new Error(`i18n: no value for {${name}} in "${text}"`)
    }
    parts.push(text.slice(last, match.index), params[name])
    last = match.index + match[0].length
  }
  parts.push(text.slice(last))
  return parts.filter((part) => part !== "")
}

/** The message under key, in the current language, with its placeholders
 * filled. A plural takes its form from params.count. */
export function t<K extends MessageKey>(
  key: K,
  ...[params]: ParamsArg<At<Messages, K>, string | number>
): string {
  const values = (params ?? {}) as Record<string, string | number>
  const count = typeof values.count === "number" ? values.count : undefined
  return interpolate(template(key, count), values).join("")
}

/** Resets the module between tests. */
export function resetLocale(): void {
  current = null
  listeners.clear()
}

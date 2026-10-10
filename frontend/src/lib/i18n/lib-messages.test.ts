import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { formatCombo, terminalCost, UNASSIGNED } from "@/lib/hotkeys"
import { noMatchNotice } from "@/lib/session/session-filter"
import { scheduleChoices, scheduledFor, timeUntil } from "@/lib/session/schedule"
import { COST_MISS_REASON } from "@/lib/session/session-cost"
import { copyToastMessage } from "@/lib/terminal/copy-toast"
import { resetLocale, setLocale } from "./i18n"

beforeEach(() => {
  const data = new Map<string, string>()
  vi.stubGlobal("localStorage", {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => data.set(key, value),
    removeItem: (key: string) => data.delete(key),
  })
  resetLocale()
})
afterEach(() => {
  vi.unstubAllGlobals()
  resetLocale()
})

describe("messages built in plain modules follow the language", () => {
  it("resolves at call time, not at import time", () => {
    const english = COST_MISS_REASON["mixed-models"]
    setLocale("pt-BR")
    expect(COST_MISS_REASON["mixed-models"]).not.toBe(english)
    expect(COST_MISS_REASON["mixed-models"]).toContain("modelo")
  })

  it("counts down in the interface language", () => {
    const now = new Date(2026, 0, 5, 12, 0, 0)
    const at = Math.floor(now.getTime() / 1000) + 45 * 60
    expect(timeUntil(at, now)).toBe("in 45m")
    setLocale("pt-BR")
    expect(timeUntil(at, now)).toMatch(/^em 45/)
    expect(scheduleChoices(now).map((choice) => choice.label)).toContain("Amanhã")
    expect(scheduledFor(at, now)).toMatch(/^hoje /)
  })

  it("keeps the placeholders of a composed sentence in order", () => {
    setLocale("pt-BR")
    expect(noMatchNotice("abc", new Set(["waiting", "unread"]))).toBe(
      "Nenhuma sessão aguardando ou não lidas corresponde a “abc”. A sessão ativa continua.",
    )
    expect(terminalCost({ mod: true, shift: false, alt: false, key: "r" })).toContain("Ctrl+R")
  })

  it("picks the plural form of the language", () => {
    setLocale("pt-BR")
    expect(copyToastMessage("a")).toBe("1 caractere copiado para a área de transferência")
    expect(copyToastMessage("ab")).toBe("2 caracteres copiados para a área de transferência")
  })

  it("leaves a chord with no cost and an unassigned one alone", () => {
    expect(terminalCost({ mod: false, shift: false, alt: false, key: "r" })).toBe("")
    expect(formatCombo(UNASSIGNED, false)).toBe("Unassigned")
  })
})

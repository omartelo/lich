import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createPullRequestPrompt, pullRequestHandoff } from "@/lib/pulls/pr-handoff"
import { delegatePrompt, delegateWorktreePrompt } from "@/lib/session/delegate-prompt"
import { composeReviewComments } from "@/lib/review-comments"
import { issueBrief } from "@/lib/issue"
import { resetPromptLanguageStore, setPromptLanguage } from "@/lib/prompt-language-store"
import { resetLocale, setLocale } from "./i18n"
import type { Issue, PullRequestDetail } from "@/lib/api-types"

vi.mock("@/lib/rpc", () => ({
  Store: { GetSetting: vi.fn().mockResolvedValue(""), SetSetting: vi.fn().mockResolvedValue(null) },
}))

const detail = (over: Partial<PullRequestDetail>): PullRequestDetail =>
  ({
    number: 7,
    state: "OPEN",
    mergeable: "MERGEABLE",
    mergeStateStatus: "CLEAN",
    baseRefName: "main",
    headRefName: "quiet-willow",
    checks: { total: 0, passed: 0, failed: 0, pending: 0 },
    checkRuns: null,
    ...over,
  }) as PullRequestDetail

const issue: Issue = {
  number: 12,
  title: "Crash on start",
  url: "https://x/y/issues/12",
  body: "",
} as Issue

beforeEach(() => {
  vi.stubGlobal("localStorage", { getItem: () => null, setItem: () => {}, removeItem: () => {} })
  resetLocale()
  resetPromptLanguageStore()
})

describe("prompt text follows the prompt language, not the interface", () => {
  it("UI en, prompt pt-BR", async () => {
    setLocale("en")
    await setPromptLanguage("pt-BR")
    expect(delegatePrompt("claude", "api")).toBe('Delegue para a sessão "api": ')
    expect(delegateWorktreePrompt("codex")).toBe("Delegue para uma nova sessão em worktree: ")
    expect(delegateWorktreePrompt("cursor")).toContain(
      "`lich open --worktree <branch> --prompt <task>`",
    )
    expect(createPullRequestPrompt("feat-x")).toContain("`gh pr create`")
    expect(createPullRequestPrompt("feat-x")).toContain("A branch feat-x ainda não tem")
    expect(composeReviewComments([])).toContain("Comentários de revisão:")
    expect(issueBrief(issue)).toContain(
      "Issue do GitHub #12: Crash on start\nhttps://x/y/issues/12",
    )
    expect(pullRequestHandoff(detail({ mergeStateStatus: "DIRTY" }))?.prompt).toContain(
      "#7 (quiet-willow) tem conflitos de merge com main",
    )
    const runs = Array.from({ length: 10 }, (_, i) => ({
      name: `job${i}`,
      state: "failed",
      url: "",
    }))
    const checks = pullRequestHandoff(
      detail({
        checks: { total: 10, passed: 0, failed: 10, pending: 0 },
        checkRuns: runs as never,
      }),
    )
    expect(checks?.prompt).toContain("…e mais 2 checks com falha não listados.")
    expect(checks?.prompt).toContain("- job0")
  })

  it("UI pt-BR, prompt en", async () => {
    setLocale("pt-BR")
    await setPromptLanguage("en")
    expect(delegatePrompt("claude", "api")).toBe('Delegate to the "api" session: ')
    expect(composeReviewComments([])).toContain("Review comments:")
    expect(pullRequestHandoff(detail({ mergeStateStatus: "DIRTY" }))?.prompt).toContain(
      "has merge conflicts with main",
    )
  })

  it("UI en, prompt es", async () => {
    setLocale("en")
    await setPromptLanguage("es")
    expect(delegatePrompt("claude", "api")).toBe('Delega en la sesión "api": ')
    expect(delegateWorktreePrompt("cursor")).toContain(
      "`lich open --worktree <branch> --prompt <task>`",
    )
    expect(createPullRequestPrompt("feat-x")).toContain("`gh pr create`")
    expect(createPullRequestPrompt("feat-x")).toContain("La rama feat-x todavía no tiene")
    expect(composeReviewComments([])).toContain("Comentarios de revisión:")
    expect(issueBrief(issue)).toContain(
      "Issue de GitHub #12: Crash on start\nhttps://x/y/issues/12",
    )
    expect(pullRequestHandoff(detail({ mergeStateStatus: "DIRTY" }))?.prompt).toContain(
      "#7 (quiet-willow) tiene conflictos de fusión con main",
    )
  })

  it("reads the language when the prompt is used, not when the handoff was made", async () => {
    const handoff = pullRequestHandoff(detail({ mergeStateStatus: "DIRTY" }))
    await setPromptLanguage("pt-BR")
    expect(handoff?.prompt).toContain("conflitos de merge")
  })
})

afterEach(() => vi.unstubAllGlobals())

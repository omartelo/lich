import { t } from "@/lib/i18n/i18n"
// The two command-line tools every version control surface runs through, and
// the page a machine without one installs it from. Both are resolved on $PATH
// exactly as the backend spawns them (internal/providers.Verify), so what this
// reports is what lich would actually run — not what a terminal would.
//
// The download pages are the vendors' own and detect the operating system
// themselves, which is why there is no per-platform URL table here to go stale.

export interface VcsTool {
  /** The binary as lich spawns it, and what Verify is asked about. */
  bin: string
  label: string
  url: string
  /** What stops working without it — the reason the row is a fact, not a nag. */
  without: string
}

export const GIT: VcsTool = {
  bin: "git",
  label: "git",
  url: "https://git-scm.com/downloads",
  get without() {
    return t("env.vcsTools.gitWithout")
  },
}

export const GH: VcsTool = {
  bin: "gh",
  label: "GitHub CLI (gh)",
  url: "https://cli.github.com",
  get without() {
    return t("env.vcsTools.ghWithout")
  },
}

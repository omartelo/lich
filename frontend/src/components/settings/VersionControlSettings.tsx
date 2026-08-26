import { useEffect } from "react"
import type { CommitIdentity } from "@/lib/api-types"
import { ProjectService } from "@/lib/rpc"
import { commitIdentityRow } from "@/lib/commit-identity"
import { accountLabel, accountSelectItems, upgradeAccount } from "@/lib/gh-account"
import { GH_ACCOUNT_KEY } from "@/lib/project-settings"
import { failed } from "@/lib/binary-layers"
import { NO_SETTLE, useBinaryCheck } from "@/lib/use-binary-check"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { GIT } from "@/lib/vcs-tools"
import { invalidatePullRequests } from "@/lib/pulls/pull-request-lookup"
import { useProjects } from "@/providers/projects"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SettingBlock } from "./SettingBlock"

// The stored value for "no override" is "", which a Select item cannot carry —
// this stands in for it in the picker only.
const ACTIVE_ACCOUNT = "__active__"
const ACTIVE_ACCOUNT_LABEL = "gh's active account"

// Module-level, as every non-primitive `empty` has to be. The two are different
// answers: null is "gh has not been asked yet", and an empty list is "gh is
// signed in to nothing", which the pane says out loud.
const NO_ACCOUNTS: string[] = []
const NO_ACCOUNTS_YET = null
const NO_IDENTITY = null

// VersionControlSettings picks which authenticated GitHub account lich's gh
// calls run as for this project. gh keeps one active account per host, so a
// repository only one of your accounts can see reads as "not found" until the
// project names that account. Project-scoped: naming one globally would be the
// same trap in the other direction.
export function VersionControlSettings({ projectId }: { projectId?: string }) {
  const { projects } = useProjects()
  const project = projects.find((p) => p.id === projectId)
  // Null until gh answers: an empty list is "gh is signed in to nothing", which
  // is a thing to say — and saying it before the first read would be a lie.
  //
  // `gh auth status` is a round trip to GitHub and the slowest read on this
  // screen, so its answer is kept for the next visit: the picker comes back
  // populated and revalidates underneath instead of emptying itself first.
  const { data: accounts, error } = useRemoteResource<string[] | null>(
    "github-accounts",
    () => ProjectService.GitHubAccounts().then((list) => list ?? NO_ACCOUNTS),
    { empty: NO_ACCOUNTS_YET, cache: "settings.githubAccounts" },
  )
  // Read once per project: git's config changes outside lich, but so rarely
  // that polling the settings page for it would buy nothing.
  const { data: identity } = useRemoteResource<CommitIdentity | null>(
    project?.path ?? "",
    () => ProjectService.CommitIdentity(project?.path ?? ""),
    { empty: NO_IDENTITY, cache: `settings.commitIdentity ${project?.path ?? ""}` },
  )
  const [account, setAccount] = useStoredSetting(GH_ACCOUNT_KEY, projectId)
  const noGit = failed(useBinaryCheck(GIT.bin, NO_SETTLE))

  // The invalidation waits for the write: it makes every pull request badge
  // look up again, and a re-read that overtook the write would replay the old
  // account's answer into the map it just cleared.
  const persist = (value: string) => {
    void setAccount(value === ACTIVE_ACCOUNT ? "" : value).then(invalidatePullRequests)
  }

  // An account stored before hosts travelled with them is rewritten the moment
  // gh says which host it lives on. It resolves either way, but leaving it
  // half-formed would show the same account twice in the picker.
  useEffect(() => {
    const upgraded = upgradeAccount(account, accounts ?? NO_ACCOUNTS)
    if (upgraded) {
      setAccount(upgraded)
    }
  }, [account, accounts, setAccount])

  if (!project) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        Open a project to configure its version control.
      </p>
    )
  }

  // The configured account survives a gh that no longer lists it, so a stale
  // value is visible (and changeable) instead of silently reading as default.
  const options = Array.from(new Set([...(accounts ?? NO_ACCOUNTS), ...(account ? [account] : [])]))
  // The account governs what lich asks gh; this says who the commits are
  // authored as. Both facts, no comparison — see commitIdentityRow. Withheld
  // without git: the empty answer git could not give reads as "this checkout
  // has no identity", which is a claim about the checkout nobody measured.
  const row = !noGit && identity && commitIdentityRow(identity)

  return (
    <SettingBlock
      title={`GitHub account for ${project.name}`}
      description={
        "Which account lich runs gh as for this project — pull requests, checks and PR checkouts. " +
        "gh keeps one active account per host, so a repository only another account can see reads " +
        "as not found. Accounts on an enterprise host are listed with it."
      }
    >
      <p className="mb-2 text-xs text-muted-foreground">{project.path}</p>
      <Select
        value={account || ACTIVE_ACCOUNT}
        items={accountSelectItems(ACTIVE_ACCOUNT, ACTIVE_ACCOUNT_LABEL, options)}
        onValueChange={(value) => value && persist(value)}
      >
        <SelectTrigger className="w-72">
          <SelectValue placeholder="Select an account" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectItem value={ACTIVE_ACCOUNT}>{ACTIVE_ACCOUNT_LABEL}</SelectItem>
            {options.map((option) => (
              <SelectItem key={option} value={option}>
                {accountLabel(option, options)}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      {/* gh's own failure already names its cause and its fix — not signed in,
          not installed, a host that would not answer — so the sentence that used
          to be appended here told half of them to run the wrong command. */}
      {error && (
        <p className="mt-2 text-xs text-destructive">Could not list GitHub accounts: {error}</p>
      )}
      {!error && accounts?.length === 0 && (
        <p className="mt-2 text-xs text-muted-foreground">
          gh is signed in to no account. Run `gh auth login` in a terminal.
        </p>
      )}
      {row && (
        <p className="mt-3 border-t border-border pt-2 text-xs text-muted-foreground">
          {row.email ? (
            <>
              {row.lead} <span className="font-mono">{row.email}</span> {row.note}
            </>
          ) : (
            <>
              <span className="text-foreground">{row.lead}</span> {row.note}
            </>
          )}
        </p>
      )}
    </SettingBlock>
  )
}

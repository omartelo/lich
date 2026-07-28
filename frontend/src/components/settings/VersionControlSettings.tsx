import { useEffect, useState } from "react"
import { ProjectService, Store } from "@/lib/rpc"
import { accountLabel, upgradeAccount } from "@/lib/gh-account"
import { GH_ACCOUNT_KEY } from "@/lib/project-settings"
import { errorText } from "@/lib/utils"
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

// VersionControlSettings picks which authenticated GitHub account lich's gh
// calls run as for this project. gh keeps one active account per host, so a
// repository only one of your accounts can see reads as "not found" until the
// project names that account. Project-scoped: naming one globally would be the
// same trap in the other direction.
export function VersionControlSettings({ projectId }: { projectId?: string }) {
  const { projects } = useProjects()
  const project = projects.find((p) => p.id === projectId)
  const [accounts, setAccounts] = useState<string[]>([])
  const [account, setAccount] = useState("")
  const [error, setError] = useState("")

  useEffect(() => {
    void ProjectService.GitHubAccounts()
      .then((list) => {
        setAccounts(list ?? [])
        setError("")
      })
      .catch((err: unknown) => setError(errorText(err)))
  }, [])

  useEffect(() => {
    if (!projectId) {
      return
    }
    void Store.GetSetting(GH_ACCOUNT_KEY, projectId).then(setAccount)
  }, [projectId])

  const persist = (value: string) => {
    const chosen = value === ACTIVE_ACCOUNT ? "" : value
    setAccount(chosen)
    if (projectId) {
      void Store.SetSetting(GH_ACCOUNT_KEY, projectId, chosen).then(invalidatePullRequests)
    }
  }

  // An account stored before hosts travelled with them is rewritten the moment
  // gh says which host it lives on. It resolves either way, but leaving it
  // half-formed would show the same account twice in the picker.
  useEffect(() => {
    if (!projectId) {
      return
    }
    const upgraded = upgradeAccount(account, accounts)
    if (upgraded) {
      setAccount(upgraded)
      void Store.SetSetting(GH_ACCOUNT_KEY, projectId, upgraded)
    }
  }, [account, accounts, projectId])

  if (!project) {
    return (
      <p className="py-4 text-sm text-muted-foreground">
        Open a project to configure its version control.
      </p>
    )
  }

  // The configured account survives a gh that no longer lists it, so a stale
  // value is visible (and changeable) instead of silently reading as default.
  const options = Array.from(new Set([...accounts, ...(account ? [account] : [])]))

  return (
    <SettingBlock
      title={`GitHub account for ${project.name}`}
      description={
        "Which account lich runs gh as for this project — pull requests, checks and PR checkouts. " +
        "gh keeps one active account per host, so a repository only another account can see reads " +
        "as not found. Accounts on an enterprise host are listed with it. Pushes are unaffected: " +
        "they still ride the remote's ssh key and the git user.email."
      }
    >
      <p className="mb-2 text-xs text-muted-foreground">{project.path}</p>
      <Select value={account || ACTIVE_ACCOUNT} onValueChange={(value) => value && persist(value)}>
        <SelectTrigger className="w-72">
          <SelectValue placeholder="Select an account" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectItem value={ACTIVE_ACCOUNT}>gh's active account</SelectItem>
            {options.map((option) => (
              <SelectItem key={option} value={option}>
                {accountLabel(option, options)}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
      {error && (
        <p className="mt-2 text-xs text-destructive">
          Could not list GitHub accounts: {error}. Run `gh auth login` to authenticate.
        </p>
      )}
    </SettingBlock>
  )
}

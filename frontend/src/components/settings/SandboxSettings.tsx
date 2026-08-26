import { System } from "@/lib/rpc"
import {
  enabledProviders,
  sandboxKey,
  sandboxLevel,
  useProviders,
  GH_TOKEN_KEY,
  SSH_AGENT_KEY,
  type SandboxLevel,
} from "@/lib/providers-store"
import { GH_ACCOUNT_KEY } from "@/lib/project-settings"
import { isMac, isWindows } from "@/lib/platform"
import { cannotConfineCopy } from "@/lib/sandbox-copy"
import { splitAccount } from "@/lib/gh-account"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { Switch } from "@/components/ui/switch"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// Why this machine cannot confine, decided once. sandbox.Backend() answers ""
// for three different reasons — no bubblewrap, no working sandbox-exec, no
// backend at all on Windows — and by the time the pane reads it, the platform is
// the only thing left that tells them apart.
const cannotConfine = cannotConfineCopy(isWindows ? "windows" : isMac ? "mac" : "linux")

// The rungs, ordered by how much of the machine a session can reach. Labels
// alone: they carry their own meaning, and the ladder is now read as a whole —
// several providers stacked and compared — rather than one consequence line at a
// time. Only "Ask" keeps a caption below, its catch not being in its name.
const RUNGS: { level: SandboxLevel; label: string }[] = [
  { level: "off", label: "Off" },
  { level: "ask", label: "Ask" },
  { level: "worktrees", label: "Worktrees" },
  { level: "everywhere", label: "Everywhere" },
]

// A module-level constant, as every array `empty` has to be: a fresh one per
// render would notify subscribers on every failed read.
const NO_KEYS: string[] = []

// useSandboxBackend names what confines a session here, "" for a machine that
// cannot. `loading` rather than a third value in `data`: the pane draws nothing
// until the answer is in, so it never flashes its "cannot confine" state, and a
// second visit has the answer already filed and paints on the first frame.
function useSandboxBackend() {
  return useRemoteResource("sandbox-backend", () => System.SandboxBackend(), {
    empty: "",
    cache: "settings.sandboxBackend",
  })
}

// useAgentKeys lists what is loaded in the user's ssh agent, read when the pane
// opens. It is the sentence the switch below it cannot say on its own: that
// switch is read as "let it push with my GitHub key", and it hands over every
// identity in this list. A key added after this read is handed over by a control
// that never named it — reopening the pane refetches, which is the cheap answer
// to that, and the filed list is only what stands on screen while it does.
function useAgentKeys(): string[] {
  const { data } = useRemoteResource(
    "ssh-agent-keys",
    // Folded rather than trusted: a machine with no agent answers null, not [],
    // and this list is read for a length.
    () => System.SSHAgentKeys().then((keys) => keys ?? NO_KEYS),
    { empty: NO_KEYS, cache: "settings.sshAgentKeys" },
  )
  return data
}

// SandboxSettings is the whole subject in one pane, asked in the order each
// question depends on the one above it: whether this machine can confine at all,
// which sessions are confined, and what a confined session may still carry in.
//
// The ladder used to sit inside every provider's own section, so a machine with
// five providers enabled drew it five times — and the one fact governing all of
// them, whether there is a backend at all, had nowhere to be said and took the
// control with it when the answer was no.
//
// scope is the project's own id when the screen was opened from a project and the
// global default from the hub, exactly the split the rung already had. Every
// control on the pane writes to it, so a grant cannot drift from the rung it
// qualifies.
export function SandboxSettings({ projectId }: { projectId?: string }) {
  const scope = projectId ?? GLOBAL_SCOPE
  const providers = useProviders()
  const { data: backend, loading, error } = useSandboxBackend()
  const agentKeys = useAgentKeys()
  const [sshAgent, setSSHAgent] = useStoredSetting(SSH_AGENT_KEY, scope)
  const [ghToken, setGHToken] = useStoredSetting(GH_TOKEN_KEY, scope)
  // The account the token will be, read from the same setting the Pulls screen
  // answers as. Naming it is the point: the switch hands over one account's
  // credentials, and which one is a project's own choice made on another pane.
  const [account] = useStoredSetting(GH_ACCOUNT_KEY, scope)

  // Nothing drawn until the machine has answered, and nothing drawn if it could
  // not: "this machine cannot confine sessions" is a claim, and a lookup that
  // failed measured nothing to back it.
  if (loading || error) {
    return null
  }

  if (backend === "") {
    return (
      <section className="py-5">
        <StatusStrip backend="" />
        <p className="mt-2 max-w-prose text-xs text-muted-foreground">{cannotConfine.advice}</p>
      </section>
    )
  }

  return (
    <>
      {/* No heading of its own: the screen is already titled Sandbox, and the
          machine's answer is the state that title is about. */}
      <section className="py-5">
        <StatusStrip backend={backend} />
        <p className="mt-2 max-w-prose text-xs text-muted-foreground">
          An empty home, the machine read-only, writes only in the checkout. The network stays on.
        </p>
      </section>

      <SettingBlock title="Which sessions run confined">
        {enabledProviders(providers).map((provider) => (
          <Rung
            key={provider.id}
            providerId={provider.id}
            providerName={provider.name}
            scope={scope}
          />
        ))}
        <p className="mt-2 max-w-prose text-xs text-muted-foreground">
          Ask reaches the New worktree dialog only.
        </p>
      </SettingBlock>

      <SettingBlock title="What a confined session may carry in">
        <Grant
          title="SSH agent"
          description="git push works inside. Signs with every identity in your agent, for any host."
          detail={agentKeys.length > 0 ? `Loaded: ${agentKeys.join(" · ")}` : "Nothing loaded."}
          checked={sshAgent === "true"}
          onChange={(next) => setSSHAgent(String(next))}
        />
        <Grant
          title="GitHub token"
          description="gh works inside. The agent can read the token out of its environment."
          detail={accountLine(account)}
          checked={ghToken === "true"}
          onChange={(next) => setGHToken(String(next))}
        />
      </SettingBlock>
    </>
  )
}

// accountLine names the account whose token a confined session would be handed.
// An unset account is gh's own active one, which is what every lich gh call
// already falls back to — and what the session would answer as, so it is said
// rather than left blank.
function accountLine(account: string): string {
  const [, login] = splitAccount(account)
  return login ? `As ${login}.` : "As gh's active account."
}

// Rung is one provider's row: its name, and the ladder for it. Compact, because
// the pane holds one per enabled provider and they are read against each other.
function Rung({
  providerId,
  providerName,
  scope,
}: {
  providerId: string
  providerName: string
  scope: string
}) {
  const [stored, setStored] = useStoredSetting(sandboxKey(providerId), scope, "off")

  return (
    <div className="flex items-center justify-between gap-4 border-t border-border py-2.5 first:border-t-0 first:pt-0">
      <span className="text-sm font-medium text-foreground">{providerName}</span>
      <ToggleGroup
        value={[sandboxLevel(stored)]}
        onValueChange={(next) => next[0] && setStored(next[0])}
        spacing={1}
        aria-label={`Which ${providerName} sessions run confined`}
        className="shrink-0 border border-border p-[3px]"
      >
        {RUNGS.map((rung) => (
          <ToggleGroupItem key={rung.level} value={rung.level} size="sm">
            {rung.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}

// StatusStrip is the machine's own answer, drawn as state rather than prose. The
// backend is named because the two have different holes, so a report about a
// confined session that names the one in play starts a round ahead.
function StatusStrip({ backend }: { backend: string }) {
  return (
    <div className="flex items-center gap-2.5 rounded-md border border-border px-3 py-2.5 text-xs">
      <span
        aria-hidden
        className={`size-[7px] shrink-0 rounded-full ${
          backend ? "bg-emerald-600 dark:bg-emerald-400" : "bg-muted-foreground"
        }`}
      />
      <span className="font-medium text-foreground">
        This machine {backend ? "can" : "cannot"} confine sessions
      </span>
      <span className="text-muted-foreground">— {backend || cannotConfine.reason}</span>
    </div>
  )
}

// Grant is one credential handed back to a confined session. The description
// carries the half nobody assumes — every identity, any host; a token the agent
// can read straight out of its environment — because without it each switch reads
// as a smaller promise than it makes.
function Grant({
  title,
  description,
  detail,
  checked,
  onChange,
}: {
  title: string
  description: string
  detail: string
  checked: boolean
  onChange: (next: boolean) => void
}) {
  return (
    <div className="flex items-start gap-4 border-t border-border py-3.5 first:border-t-0 first:pt-0">
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-foreground">{title}</div>
        <p className="mt-0.5 max-w-prose text-xs text-muted-foreground">{description}</p>
        <p className="mt-1 font-mono text-[11px] leading-relaxed text-muted-foreground">{detail}</p>
      </div>
      <Switch checked={checked} onCheckedChange={onChange} aria-label={title} />
    </div>
  )
}

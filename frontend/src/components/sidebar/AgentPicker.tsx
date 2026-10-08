import { ProviderIcon } from "@/components/ProviderIcon"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import type { ProviderState } from "@/lib/providers-store"
import { canRace } from "@/lib/session/agent-race"
import type { ProviderKind } from "@/lib/session/sessions"
import { cn } from "@/lib/utils"

// "Crush", "Antigravity and Crush", "Antigravity, opencode and Crush".
const listed = (names: string[]): string =>
  names.length < 2
    ? names.join("")
    : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`

interface AgentPickerProps {
  label: string
  /** The providers New session offers, in its order. */
  providers: ProviderState[]
  picked: ProviderKind[]
  onToggle: (kind: ProviderKind) => void
}

// AgentPicker is the row of providers a race or a consolidation runs. A
// provider that cannot race is drawn dead with the reason under the row,
// SessionForkItem's idiom: a user who finds Crush missing has no way to learn
// that it was withheld rather than forgotten.
export function AgentPicker({ label, providers, picked, onToggle }: AgentPickerProps) {
  const out = providers.filter((provider) => !canRace(provider.id))
  return (
    <div className="flex flex-col gap-1.5">
      <Label className="text-xs uppercase tracking-wide">{label}</Label>
      <div className="flex flex-wrap gap-1">
        {providers.map((provider) => {
          const on = picked.includes(provider.id)
          const dead = !canRace(provider.id)
          return (
            // biome-ignore lint/a11y/noLabelWithoutControl: the Checkbox inside is the control; base-ui renders it as a span the wrapping label activates
            <label
              key={provider.id}
              className={cn(
                "flex items-center gap-1.5 rounded-md py-1 pr-2.5 pl-1.5 text-sm transition-colors",
                dead && "opacity-50",
                !dead && "cursor-pointer",
                on ? "bg-accent text-accent-foreground" : "text-muted-foreground",
                !on && !dead && "hover:bg-accent/50",
              )}
            >
              <Checkbox
                checked={on}
                disabled={dead}
                onCheckedChange={() => onToggle(provider.id)}
              />
              <ProviderIcon kind={provider.id} size={14} />
              {provider.name}
            </label>
          )
        })}
      </div>
      {out.length > 0 && (
        <span className="text-xs text-muted-foreground">
          {listed(out.map((provider) => provider.name))} cannot run here:{" "}
          {out.length === 1 ? "its" : "their"} first start in a worktree asks a question lich cannot
          see past.
        </span>
      )}
    </div>
  )
}

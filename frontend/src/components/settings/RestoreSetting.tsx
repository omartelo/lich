import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { restoreChoice, restoreKey, type RestoreChoice } from "@/lib/providers-store"
import { canResume } from "@/lib/session/sessions"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

const RESTORE_CHOICES: { choice: RestoreChoice; label: string; consequence: string }[] = [
  {
    choice: "ask",
    label: "Ask",
    consequence: "Every restored card asks before it opens.",
  },
  {
    choice: "resume",
    label: "Resume",
    consequence:
      "Restored cards continue their conversation on open. For an empty one, open a new session.",
  },
  {
    choice: "fresh",
    label: "Start new",
    consequence: "Restored cards open empty. The old conversation is not picked up.",
  },
]

// RestoreSetting answers the resume prompt ahead of time for one provider. It
// only covers a conversation that is still there: one that is gone opens empty
// with a notice whatever is chosen here.
export function RestoreSetting({
  providerId,
  providerName,
}: {
  providerId: string
  providerName: string
}) {
  const [stored, persist] = useStoredSetting(restoreKey(providerId), GLOBAL_SCOPE)
  if (!canResume(providerId)) {
    return null
  }
  const choice = restoreChoice(stored)
  const consequence = RESTORE_CHOICES.find((c) => c.choice === choice)?.consequence ?? ""

  return (
    <SettingBlock
      title="Restored sessions"
      description={`What a card that ran ${providerName} before lich restarted does the first time you open it.`}
    >
      <ToggleGroup
        value={[choice]}
        // An empty array is the pressed choice pressed again: keep it.
        onValueChange={(next) => next[0] && void persist(next[0])}
        spacing={1}
        aria-label={`What a restored ${providerName} card does`}
        className="border border-border p-[0.1875rem]"
      >
        {RESTORE_CHOICES.map((c) => (
          <ToggleGroupItem key={c.choice} value={c.choice} size="sm">
            {c.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <p className="mt-2 text-xs text-muted-foreground">{consequence}</p>
    </SettingBlock>
  )
}

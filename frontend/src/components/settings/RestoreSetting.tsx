import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { restoreChoice, restoreKey, type RestoreChoice } from "@/lib/providers-store"
import { canResume } from "@/lib/session/sessions"
import { useT } from "@/lib/i18n/i18n"
import { useStoredSetting } from "@/lib/use-stored-setting"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

const RESTORE_CHOICES: RestoreChoice[] = ["ask", "resume", "fresh"]

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
  const t = useT()
  const [stored, persist] = useStoredSetting(restoreKey(providerId), GLOBAL_SCOPE)
  if (!canResume(providerId)) {
    return null
  }
  const choice = restoreChoice(stored)
  const consequence = t(`settings.restoreSetting.choice.${choice}.consequence`)

  return (
    <SettingBlock
      title={t("settings.restoreSetting.title")}
      description={t("settings.restoreSetting.description", { provider: providerName })}
    >
      <ToggleGroup
        value={[choice]}
        // An empty array is the pressed choice pressed again: keep it.
        onValueChange={(next) => next[0] && void persist(next[0])}
        spacing={1}
        aria-label={t("settings.restoreSetting.label", { provider: providerName })}
        className="border border-border p-[0.1875rem]"
      >
        {RESTORE_CHOICES.map((c) => (
          <ToggleGroupItem key={c} value={c} size="sm">
            {t(`settings.restoreSetting.choice.${c}.label`)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <p className="mt-2 text-xs text-muted-foreground">{consequence}</p>
    </SettingBlock>
  )
}

import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { DIFF_LAYOUTS, type DiffLayout } from "@/lib/git/diff-layout"
import { useT } from "@/lib/i18n/i18n"
import { useSettings } from "@/providers/settings"
import { SettingRow } from "./SettingBlock"

// DiffLayoutSetting picks how every diff draws, the Review tab's and a pull
// request's. It is the only place the choice is made: the panels draw what was
// chosen and carry no switch of their own.
export function DiffLayoutSetting() {
  const t = useT()
  const { diffLayout, setDiffLayout } = useSettings()
  return (
    <SettingRow
      title={t("settings.diffLayoutSetting.title")}
      description={t("settings.diffLayoutSetting.description")}
    >
      <ToggleGroup
        value={[diffLayout]}
        onValueChange={(next) => next[0] && setDiffLayout(next[0] as DiffLayout)}
        spacing={1}
        aria-label={t("settings.diffLayoutSetting.title")}
        className="shrink-0 border border-border p-[0.1875rem]"
      >
        {DIFF_LAYOUTS.map((layout) => (
          <ToggleGroupItem key={layout} value={layout} size="sm">
            {t(`settings.diffLayoutSetting.layout.${layout}`)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </SettingRow>
  )
}

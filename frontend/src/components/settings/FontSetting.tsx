import { useMemo } from "react"
import { useT } from "@/lib/i18n/i18n"
import { Fonts as FontService } from "@/lib/rpc"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { DEFAULT_FONT, useSettings } from "@/providers/settings"
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxTrigger,
  ComboboxValue,
} from "@/components/ui/combobox"
import { Button } from "@/components/ui/button"
import { SettingRow } from "./SettingBlock"

// A module-level constant, as every array `empty` has to be: a fresh one per
// render would notify subscribers on every failed read.
const NO_FAMILIES: string[] = []

export function FontSetting() {
  const t = useT()
  const { font, setFont } = useSettings()
  // Kept for the next visit: this is fontconfig's whole roster, and it is the
  // same answer every time — a picker that empties itself back to two entries
  // on the way in has nothing to gain by re-asking first.
  const { data: families } = useRemoteResource(
    "font-families",
    () => FontService.List().then((list) => list ?? NO_FAMILIES),
    { empty: NO_FAMILIES, cache: "settings.fontFamilies" },
  )

  // Always offer the bundled default and the current selection, even if
  // fontconfig does not list them (the bundled font is not OS-installed).
  const options = useMemo(
    () => Array.from(new Set([DEFAULT_FONT, font, ...families])),
    [families, font],
  )

  return (
    <SettingRow
      title={t("settings.fontSetting.title")}
      description={
        // The sample renders in the family itself: the answer to "which one is
        // this" is the shape of the glyphs, not the name.
        <span style={{ fontFamily: font }}>{t("settings.fontSetting.sample")}</span>
      }
    >
      <Combobox items={options} value={font} onValueChange={(value) => value && setFont(value)}>
        <ComboboxTrigger
          render={<Button variant="outline" className="w-64 justify-between font-normal" />}
          aria-label={t("settings.fontSetting.title")}
        >
          <span className="truncate">
            <ComboboxValue />
          </span>
        </ComboboxTrigger>
        <ComboboxContent>
          <ComboboxInput showTrigger={false} placeholder={t("settings.fontSetting.search")} />
          <ComboboxEmpty>{t("settings.fontSetting.empty")}</ComboboxEmpty>
          <ComboboxList>
            {(family: string) => (
              <ComboboxItem key={family} value={family} title={family}>
                <span className="truncate">{family}</span>
              </ComboboxItem>
            )}
          </ComboboxList>
        </ComboboxContent>
      </Combobox>
    </SettingRow>
  )
}

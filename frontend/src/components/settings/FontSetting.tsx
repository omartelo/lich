import { useMemo } from "react"
import { Fonts as FontService } from "@/lib/rpc"
import { useRemoteResource } from "@/lib/use-remote-resource"
import { DEFAULT_FONT, useSettings } from "@/providers/settings"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SettingBlock } from "./SettingBlock"

// A module-level constant, as every array `empty` has to be: a fresh one per
// render would notify subscribers on every failed read.
const NO_FAMILIES: string[] = []

export function FontSetting() {
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
    <SettingBlock title="Font" description="Font family used to render the terminal.">
      <Select value={font} onValueChange={(value) => value && setFont(value)}>
        <SelectTrigger className="w-64">
          <SelectValue placeholder="Select a font" />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {options.map((family) => (
              <SelectItem key={family} value={family}>
                {family}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </SettingBlock>
  )
}

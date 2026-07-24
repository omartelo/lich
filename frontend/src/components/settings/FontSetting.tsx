import { useEffect, useMemo, useState } from "react"
import { Fonts as FontService } from "@/lib/rpc"
import { DEFAULT_FONT, useSettings } from "@/lib/settings"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SettingBlock } from "./SettingBlock"

export function FontSetting() {
  const { font, setFont } = useSettings()
  const [families, setFamilies] = useState<string[]>([])

  useEffect(() => {
    void FontService.List()
      .then((list) => setFamilies(list ?? []))
      .catch(() => setFamilies([]))
  }, [])

  // Always offer the bundled default and the current selection, even if
  // fontconfig does not list them (the bundled font is not OS-installed).
  const options = useMemo(
    () => Array.from(new Set([DEFAULT_FONT, font, ...families])),
    [families, font],
  )

  return (
    <SettingBlock
      title="Font"
      description="Font family used to render the terminal."
    >
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

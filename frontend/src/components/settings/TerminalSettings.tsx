import { useEffect, useMemo, useState } from "react"
import { Service as FontService } from "../../../bindings/github.com/skipodotdev/skipo/internals/fonts"
import { DEFAULT_FONT, useSettings } from "@/lib/settings"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SettingRow } from "./SettingRow"

export function TerminalSettings() {
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
    <div>
      <h1 className="mb-6 text-2xl font-semibold text-foreground">Terminal</h1>
      <div className="border-t border-border">
        <SettingRow
          label="Font"
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
        </SettingRow>
      </div>
    </div>
  )
}

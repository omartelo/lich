import { THEMES, useSettings } from "@/lib/settings"
import type { Theme } from "@/lib/settings"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { SettingRow } from "./SettingRow"

const THEME_LABELS: Record<Theme, string> = {
  system: "System",
  light: "Light",
  dark: "Dark",
}

export function AppearanceSettings() {
  const { theme, setTheme } = useSettings()

  return (
    <div>
      <h1 className="mb-6 text-2xl font-semibold text-foreground">Appearance</h1>
      <div className="border-t border-border">
        <SettingRow label="Theme" description="Color theme used across the app.">
          <Select
            value={theme}
            onValueChange={(value) => value && setTheme(value as Theme)}
          >
            <SelectTrigger className="w-64">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {THEMES.map((value) => (
                  <SelectItem key={value} value={value}>
                    {THEME_LABELS[value]}
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

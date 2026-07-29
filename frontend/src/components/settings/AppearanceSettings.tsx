import {
  CaseSensitive,
  Minus,
  Plus,
  SquareTerminal,
  Trash2,
  Upload,
  ZoomIn,
  ZoomOut,
} from "lucide-react"
import { useRef } from "react"
import { toast } from "sonner"
import {
  DEFAULT_TERMINAL_FONT_SIZE,
  DEFAULT_ZOOM,
  TERMINAL_FONT_SIZE_MAX,
  TERMINAL_FONT_SIZE_MIN,
  TERMINAL_FONT_SIZE_STEP,
  ZOOM_MAX,
  ZOOM_MIN,
  ZOOM_STEP,
  useSettings,
} from "@/providers/settings"
import type { TerminalTheme, Theme } from "@/providers/settings"
import { Stepper } from "@/components/common/Stepper"
import { SettingBlock, SettingGroup } from "./SettingBlock"
import { FontSetting } from "./FontSetting"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { customTheme } from "@/lib/themes"
import { errorText } from "@/lib/utils"

// Appearance holds every look-and-feel control, split into an Interface group
// (theme, zoom) and a Terminal group (background, text size, font) so the two
// concerns read apart instead of as one flat list. The group label supplies the
// context, so the block titles drop their "Interface"/"Terminal" prefix.
export function AppearanceSettings() {
  const fileRef = useRef<HTMLInputElement | null>(null)
  const {
    themes,
    theme,
    setTheme,
    importTheme,
    removeTheme,
    zoom,
    setZoom,
    terminalFontSize,
    setTerminalFontSize,
    terminalTheme,
    setTerminalTheme,
  } = useSettings()
  const removableTheme = customTheme(themes, theme)

  const onImportTheme = async (file: File | undefined) => {
    if (!file) {
      return
    }
    try {
      const imported = await importTheme(await file.text())
      toast.success(`Imported theme: ${imported.name}`)
    } catch (error) {
      toast.error(`Theme import failed: ${errorText(error)}`)
    } finally {
      if (fileRef.current) {
        fileRef.current.value = ""
      }
    }
  }

  const onRemoveTheme = async () => {
    if (!removableTheme) {
      return
    }
    try {
      await removeTheme(removableTheme.id)
      toast.success(`Removed theme: ${removableTheme.name}`)
    } catch (error) {
      toast.error(`Theme removal failed: ${errorText(error)}`)
    }
  }

  return (
    <>
      <SettingGroup label="Interface">
        <SettingBlock
          title="Theme"
          description="Controls the app color tokens. System follows your OS and uses the bundled light or dark theme."
        >
          <div className="flex flex-wrap items-center gap-2">
            <Select value={theme} onValueChange={(value) => value && setTheme(value as Theme)}>
              <SelectTrigger className="w-64">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectLabel>Automatic</SelectLabel>
                  <SelectItem value="system">System</SelectItem>
                </SelectGroup>
                <SelectGroup>
                  <SelectLabel>Bundled</SelectLabel>
                  {themes
                    .filter((item) => item.origin === "bundled")
                    .map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                </SelectGroup>
                {themes.some((item) => item.origin === "custom") && (
                  <SelectGroup>
                    <SelectLabel>Custom</SelectLabel>
                    {themes
                      .filter((item) => item.origin === "custom")
                      .map((item) => (
                        <SelectItem key={item.id} value={item.id}>
                          {item.name}
                        </SelectItem>
                      ))}
                  </SelectGroup>
                )}
              </SelectContent>
            </Select>
            <input
              ref={fileRef}
              type="file"
              accept="application/json,.json"
              className="hidden"
              onChange={(event) => void onImportTheme(event.currentTarget.files?.[0])}
            />
            <Button type="button" variant="outline" onClick={() => fileRef.current?.click()}>
              <Upload />
              Import JSON
            </Button>
            <Button
              type="button"
              variant="ghost"
              disabled={!removableTheme}
              onClick={() => void onRemoveTheme()}
            >
              <Trash2 />
              Remove
            </Button>
          </div>
        </SettingBlock>

        <SettingBlock
          icon={<ZoomIn className="size-4" />}
          title="Zoom"
          description="Scales the interface."
        >
          <Stepper
            value={zoom}
            display={`${Math.round(zoom * 100)}%`}
            min={ZOOM_MIN}
            max={ZOOM_MAX}
            step={ZOOM_STEP}
            fallback={DEFAULT_ZOOM}
            onChange={setZoom}
            decrementIcon={<ZoomOut />}
            incrementIcon={<ZoomIn />}
            decrementLabel="Zoom out"
            incrementLabel="Zoom in"
          />
        </SettingBlock>
      </SettingGroup>

      <SettingGroup label="Terminal">
        <SettingBlock
          icon={<SquareTerminal className="size-4" />}
          title="Theme"
          description="Match app keeps the terminal in sync with the interface theme so text stays legible."
        >
          <Select
            value={terminalTheme}
            onValueChange={(value) => value && setTerminalTheme(value as TerminalTheme)}
          >
            <SelectTrigger className="w-64">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectLabel>Automatic</SelectLabel>
                <SelectItem value="match">Match app</SelectItem>
              </SelectGroup>
              <SelectGroup>
                <SelectLabel>Bundled</SelectLabel>
                {themes
                  .filter((item) => item.origin === "bundled")
                  .map((item) => (
                    <SelectItem key={item.id} value={item.id}>
                      {item.name}
                    </SelectItem>
                  ))}
              </SelectGroup>
              {themes.some((item) => item.origin === "custom") && (
                <SelectGroup>
                  <SelectLabel>Custom</SelectLabel>
                  {themes
                    .filter((item) => item.origin === "custom")
                    .map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                </SelectGroup>
              )}
            </SelectContent>
          </Select>
        </SettingBlock>

        <SettingBlock
          icon={<CaseSensitive className="size-4" />}
          title="Text size"
          description="Scales the terminal."
        >
          <Stepper
            value={terminalFontSize}
            display={`${terminalFontSize}px`}
            min={TERMINAL_FONT_SIZE_MIN}
            max={TERMINAL_FONT_SIZE_MAX}
            step={TERMINAL_FONT_SIZE_STEP}
            fallback={DEFAULT_TERMINAL_FONT_SIZE}
            onChange={setTerminalFontSize}
            decrementIcon={<Minus />}
            incrementIcon={<Plus />}
            decrementLabel="Smaller terminal text"
            incrementLabel="Larger terminal text"
          />
        </SettingBlock>

        <FontSetting />
      </SettingGroup>
    </>
  )
}

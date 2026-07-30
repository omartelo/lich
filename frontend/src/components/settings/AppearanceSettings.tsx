import {
  CaseSensitive,
  ChevronDown,
  Download,
  Minus,
  Plus,
  SquareTerminal,
  Trash2,
  Upload,
  ZoomIn,
  ZoomOut,
} from "lucide-react"
import { useRef, useState } from "react"
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
import type { ThemeDefinition } from "@/lib/api-types"
import { ConfirmDialog } from "@/components/ConfirmDialog"
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
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { customThemes, THEME_TEMPLATE_FILENAME, themeTemplateJSON } from "@/lib/themes"
import { errorText } from "@/lib/utils"

// Appearance holds every look-and-feel control, split into an Interface group
// (theme, zoom) and a Terminal group (background, text size, font) so the two
// concerns read apart instead of as one flat list. The group label supplies the
// context, so the block titles drop their "Interface"/"Terminal" prefix.
export function AppearanceSettings() {
  const fileRef = useRef<HTMLInputElement | null>(null)
  const [themePendingRemoval, setThemePendingRemoval] = useState<ThemeDefinition | null>(null)
  const [importedThemesOpen, setImportedThemesOpen] = useState(false)
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
  const importedThemes = customThemes(themes)
  const hasCustomThemes = importedThemes.length > 0

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
    if (!themePendingRemoval) {
      return
    }
    try {
      await removeTheme(themePendingRemoval.id)
      toast.success(`Removed theme: ${themePendingRemoval.name}`)
      setThemePendingRemoval(null)
    } catch (error) {
      toast.error(`Theme removal failed: ${errorText(error)}`)
    }
  }

  const onDownloadThemeTemplate = () => {
    const url = URL.createObjectURL(new Blob([themeTemplateJSON()], { type: "application/json" }))
    const link = document.createElement("a")
    link.href = url
    link.download = THEME_TEMPLATE_FILENAME
    document.body.append(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    toast.success("Template download started!")
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
                {hasCustomThemes && (
                  <SelectGroup>
                    <SelectLabel>Custom</SelectLabel>
                    {importedThemes.map((item) => (
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
              Import
            </Button>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Download Theme Template"
                    onClick={onDownloadThemeTemplate}
                  />
                }
              >
                <Download />
              </TooltipTrigger>
              <TooltipContent>Download Theme Template</TooltipContent>
            </Tooltip>
          </div>
          {hasCustomThemes && (
            <div className="mt-3 w-full max-w-md">
              <Button
                type="button"
                variant="ghost"
                className="h-8 w-full justify-start px-1.5 text-muted-foreground"
                aria-expanded={importedThemesOpen}
                onClick={() => setImportedThemesOpen((open) => !open)}
              >
                <ChevronDown
                  className={
                    importedThemesOpen ? "transition-transform" : "-rotate-90 transition-transform"
                  }
                />
                Imported themes ({importedThemes.length})
              </Button>
              {importedThemesOpen && (
                <div className="mt-1 space-y-0.5">
                  {importedThemes.map((item) => (
                    <div
                      key={item.id}
                      className="flex min-h-10 items-center gap-2 rounded-md px-2 py-1.5 hover:bg-accent/50"
                    >
                      <span
                        aria-hidden
                        className="size-4 shrink-0 rounded-sm border border-border"
                        style={{ backgroundColor: item.app.background }}
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm">{item.name}</span>
                        <span className="block truncate font-mono text-xs text-muted-foreground">
                          {item.id}
                        </span>
                      </span>
                      <Tooltip>
                        <TooltipTrigger
                          render={
                            <Button
                              type="button"
                              variant="ghost"
                              size="icon-sm"
                              aria-label={`Remove ${item.name}`}
                              onClick={() => setThemePendingRemoval(item)}
                            />
                          }
                        >
                          <Trash2 />
                        </TooltipTrigger>
                        <TooltipContent>{`Remove ${item.name}`}</TooltipContent>
                      </Tooltip>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
          <ConfirmDialog
            open={themePendingRemoval !== null}
            onCancel={() => setThemePendingRemoval(null)}
            title="Remove imported theme?"
            description={
              <>
                Delete <span className="font-medium">{themePendingRemoval?.name}</span>? This
                removes the imported theme file from lich.
              </>
            }
          >
            <Button variant="destructive" onClick={() => void onRemoveTheme()}>
              Remove theme
            </Button>
          </ConfirmDialog>
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
              {hasCustomThemes && (
                <SelectGroup>
                  <SelectLabel>Custom</SelectLabel>
                  {importedThemes.map((item) => (
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

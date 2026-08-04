import {
  CaseSensitive,
  ChevronDown,
  Download,
  GitBranch,
  Minus,
  Plus,
  RefreshCw,
  SquareTerminal,
  Trash2,
  Upload,
  ZoomIn,
  ZoomOut,
} from "lucide-react"
import { useState } from "react"
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
import { ProjectService, Themes } from "@/lib/rpc"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { Stepper } from "@/components/common/Stepper"
import { SettingBlock, SettingGroup } from "./SettingBlock"
import { FontSetting } from "./FontSetting"
import { ImportThemeDialog } from "./ImportThemeDialog"
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
import {
  bundledThemes,
  customThemes,
  MATCH_TERMINAL_THEME,
  repoLabel,
  SYSTEM_THEME,
  THEME_TEMPLATE_FILENAME,
  themeSelectItems,
} from "@/lib/themes"
import { errorText } from "@/lib/utils"

// Appearance holds every look-and-feel control, split into an Interface group
// (theme, zoom) and a Terminal group (background, text size, font) so the two
// concerns read apart instead of as one flat list. The group label supplies the
// context, so the block titles drop their "Interface"/"Terminal" prefix.
export function AppearanceSettings() {
  const [themePendingRemoval, setThemePendingRemoval] = useState<ThemeDefinition | null>(null)
  const [themePendingOverwrite, setThemePendingOverwrite] = useState<{
    path: string
    theme: ThemeDefinition
  } | null>(null)
  const [packPendingOverwrite, setPackPendingOverwrite] = useState<{
    url: string
    conflicts: string[]
  } | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [importing, setImporting] = useState(false)
  const [updatingID, setUpdatingID] = useState<string | null>(null)
  const [importedThemesOpen, setImportedThemesOpen] = useState(false)
  const {
    themes,
    theme,
    setTheme,
    importTheme,
    installThemesFromGit,
    updateThemeFromGit,
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

  const onChooseThemeFile = async () => {
    setImporting(true)
    try {
      const path = await ProjectService.PickFile("Import Theme")
      if (!path) return
      const result = await importTheme(path, false)
      setImportOpen(false)
      if (result.needsOverwrite) {
        setThemePendingOverwrite({ path, theme: result.theme })
        return
      }
      toast.success(`Imported theme: ${result.theme.name}`)
    } catch (error) {
      toast.error(`Theme import failed: ${errorText(error)}`)
    } finally {
      setImporting(false)
    }
  }

  const installRepository = async (url: string, overwrite: boolean) => {
    setImporting(true)
    try {
      const result = await installThemesFromGit(url, overwrite)
      setImportOpen(false)
      const conflicts = result.conflicts ?? []
      if (conflicts.length > 0) {
        setPackPendingOverwrite({ url, conflicts })
        return
      }
      setPackPendingOverwrite(null)
      toast.success(
        `Installed ${themeCount(result.themes?.length ?? 0)} from ${result.pack} v${result.version}`,
      )
    } catch (error) {
      toast.error(`Theme install failed: ${errorText(error)}`)
    } finally {
      setImporting(false)
    }
  }

  const onUpdateTheme = async (item: ThemeDefinition) => {
    setUpdatingID(item.id)
    try {
      const result = await updateThemeFromGit(item.id)
      toast.success(
        result.upToDate
          ? `${result.pack} is already at v${result.version}`
          : `Updated ${result.pack} to v${result.version}`,
      )
    } catch (error) {
      toast.error(`Theme update failed: ${errorText(error)}`)
    } finally {
      setUpdatingID(null)
    }
  }

  const onOverwriteTheme = async () => {
    if (!themePendingOverwrite) return
    try {
      const result = await importTheme(themePendingOverwrite.path, true)
      toast.success(`Imported theme: ${result.theme.name}`)
      setThemePendingOverwrite(null)
    } catch (error) {
      toast.error(`Theme import failed: ${errorText(error)}`)
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

  const onDownloadThemeTemplate = async () => {
    try {
      const path = await ProjectService.PickSaveFile("Save Theme Template", THEME_TEMPLATE_FILENAME)
      if (!path) return
      await Themes.SaveTemplate(path)
      toast.success(`Saved theme template to ${path}`)
    } catch (error) {
      toast.error(`Theme template failed: ${errorText(error)}`)
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
            <Select
              value={theme}
              items={themeSelectItems(themes, SYSTEM_THEME, "System")}
              onValueChange={(value) => value && setTheme(value as Theme)}
            >
              <SelectTrigger className="w-64">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  <SelectLabel>Automatic</SelectLabel>
                  <SelectItem value={SYSTEM_THEME}>System</SelectItem>
                </SelectGroup>
                <ThemeOptions themes={themes} />
              </SelectContent>
            </Select>
            <Button type="button" variant="outline" onClick={() => setImportOpen(true)}>
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
                    onClick={() => void onDownloadThemeTemplate()}
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
                        {/* The row is narrow: the repository replaces the id
                            rather than sharing the line, or both truncate away. */}
                        {item.source ? (
                          <span className="flex min-w-0 items-center gap-1.5 font-mono text-xs text-muted-foreground">
                            <GitBranch className="size-3 shrink-0" />
                            <span className="truncate" title={item.source.url}>
                              {repoLabel(item.source.url)}
                            </span>
                            <span className="shrink-0 tabular-nums">v{item.source.version}</span>
                          </span>
                        ) : (
                          <span className="block truncate font-mono text-xs text-muted-foreground">
                            {item.id}
                          </span>
                        )}
                      </span>
                      {item.source && (
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon-sm"
                                aria-label={`Update ${item.name}`}
                                disabled={updatingID !== null}
                                onClick={() => void onUpdateTheme(item)}
                              />
                            }
                          >
                            <RefreshCw className={updatingID === item.id ? "animate-spin" : ""} />
                          </TooltipTrigger>
                          <TooltipContent>Check the repository for a newer version</TooltipContent>
                        </Tooltip>
                      )}
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
          <ImportThemeDialog
            open={importOpen}
            onOpenChange={setImportOpen}
            onInstallRepository={(url) => installRepository(url, false)}
            onChooseFile={onChooseThemeFile}
            busy={importing}
          />
          <ConfirmDialog
            open={packPendingOverwrite !== null}
            onCancel={() => setPackPendingOverwrite(null)}
            title="Replace imported themes?"
            description={
              <>
                The repository carries themes that are already installed:{" "}
                <span className="font-medium">{packPendingOverwrite?.conflicts.join(", ")}</span>.
                Install anyway? This permanently deletes the previous versions.
              </>
            }
          >
            <Button
              variant="destructive"
              disabled={importing}
              onClick={() => {
                if (packPendingOverwrite) void installRepository(packPendingOverwrite.url, true)
              }}
            >
              Replace themes
            </Button>
          </ConfirmDialog>
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
          <ConfirmDialog
            open={themePendingOverwrite !== null}
            onCancel={() => setThemePendingOverwrite(null)}
            title="Replace imported theme?"
            description={
              <>
                A theme with the id{" "}
                <span className="font-medium">{themePendingOverwrite?.theme.id}</span> already
                exists. Import anyway? This permanently deletes the previous theme.
              </>
            }
          >
            <Button variant="destructive" onClick={() => void onOverwriteTheme()}>
              Replace theme
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
            items={themeSelectItems(themes, MATCH_TERMINAL_THEME, "Match app")}
            onValueChange={(value) => value && setTerminalTheme(value as TerminalTheme)}
          >
            <SelectTrigger className="w-64">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                <SelectLabel>Automatic</SelectLabel>
                <SelectItem value={MATCH_TERMINAL_THEME}>Match app</SelectItem>
              </SelectGroup>
              <ThemeOptions themes={themes} />
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

function themeCount(count: number): string {
  return count === 1 ? "1 theme" : `${count} themes`
}

interface ThemeOptionsProps {
  themes: readonly ThemeDefinition[]
}

function ThemeOptions({ themes }: ThemeOptionsProps) {
  const bundled = bundledThemes(themes)
  const custom = customThemes(themes)
  return (
    <>
      <SelectGroup>
        <SelectLabel>Bundled</SelectLabel>
        {bundled.map((item) => (
          <SelectItem key={item.id} value={item.id}>
            {item.name}
          </SelectItem>
        ))}
      </SelectGroup>
      {custom.length > 0 && (
        <SelectGroup>
          <SelectLabel>Custom</SelectLabel>
          {custom.map((item) => (
            <SelectItem key={item.id} value={item.id}>
              {item.name}
            </SelectItem>
          ))}
        </SelectGroup>
      )}
    </>
  )
}

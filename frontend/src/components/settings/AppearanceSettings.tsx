import { Minus, Plus, ZoomIn, ZoomOut } from "lucide-react"
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
import type { Theme } from "@/providers/settings"
import type { ThemeDefinition } from "@/lib/api-types"
import { ProjectService, Themes } from "@/lib/rpc"
import { ConfirmDialog } from "@/components/ConfirmDialog"
import { Stepper } from "@/components/common/Stepper"
import { SettingRow } from "./SettingBlock"
import { FontSetting } from "./FontSetting"
import { FooterSettings } from "./FooterSettings"
import { ImportThemeDialog } from "./ImportThemeDialog"
import { ThemePicker } from "./ThemePicker"
import { Button } from "@/components/ui/button"
import { THEME_TEMPLATE_FILENAME } from "@/lib/themes"
import { errorText } from "@/lib/utils"

// Appearance is a list of rows: a name on the left, its control on the right.
// Two of them carry more than a control — the theme opens a strip of previews,
// the footer an editor — and both open in place, below their own row, so the
// pane never stops being one list.
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
  const {
    themes,
    theme,
    resolvedTheme,
    setTheme,
    importTheme,
    installThemesFromGit,
    updateThemeFromGit,
    removeTheme,
    zoom,
    setZoom,
    terminalFontSize,
    setTerminalFontSize,
  } = useSettings()

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
      <ThemePicker
        themes={themes}
        value={theme}
        resolved={resolvedTheme}
        onSelect={(id: Theme) => setTheme(id)}
        onImport={() => setImportOpen(true)}
        onUpdate={(item) => void onUpdateTheme(item)}
        onRemove={setThemePendingRemoval}
        updatingID={updatingID}
      />

      <SettingRow title="Zoom">
        <Stepper
          value={zoom}
          display={`${Math.round(zoom * 100)}%`}
          min={ZOOM_MIN}
          max={ZOOM_MAX}
          step={ZOOM_STEP}
          fallback={DEFAULT_ZOOM}
          name="the zoom"
          onChange={setZoom}
          decrementIcon={<ZoomOut />}
          incrementIcon={<ZoomIn />}
          decrementLabel="Zoom out"
          incrementLabel="Zoom in"
        />
      </SettingRow>

      <SettingRow title="Terminal text size">
        <Stepper
          value={terminalFontSize}
          display={`${terminalFontSize}px`}
          min={TERMINAL_FONT_SIZE_MIN}
          max={TERMINAL_FONT_SIZE_MAX}
          step={TERMINAL_FONT_SIZE_STEP}
          fallback={DEFAULT_TERMINAL_FONT_SIZE}
          name="the terminal text size"
          onChange={setTerminalFontSize}
          decrementIcon={<Minus />}
          incrementIcon={<Plus />}
          decrementLabel="Smaller terminal text"
          incrementLabel="Larger terminal text"
        />
      </SettingRow>

      <FontSetting />
      <FooterSettings />

      <ImportThemeDialog
        open={importOpen}
        onOpenChange={setImportOpen}
        onInstallRepository={(url) => installRepository(url, false)}
        onChooseFile={onChooseThemeFile}
        onDownloadTemplate={() => void onDownloadThemeTemplate()}
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
            Delete <span className="font-medium">{themePendingRemoval?.name}</span>? This removes
            the imported theme file from lich.
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
            <span className="font-medium">{themePendingOverwrite?.theme.id}</span> already exists.
            Import anyway? This permanently deletes the previous theme.
          </>
        }
      >
        <Button variant="destructive" onClick={() => void onOverwriteTheme()}>
          Replace theme
        </Button>
      </ConfirmDialog>
    </>
  )
}

function themeCount(count: number): string {
  return count === 1 ? "1 theme" : `${count} themes`
}

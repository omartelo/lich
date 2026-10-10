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
import { LanguageSettings } from "./LanguageSettings"
import { ImportThemeDialog } from "./ImportThemeDialog"
import { ThemePicker } from "./ThemePicker"
import { Button } from "@/components/ui/button"
import { Trans } from "@/components/common/Trans"
import { useT } from "@/lib/i18n/i18n"
import { THEME_TEMPLATE_FILENAME } from "@/lib/themes"
import { errorText } from "@/lib/utils"

// Appearance is a list of rows: a name on the left, its control on the right.
// Two of them carry more than a control — the theme opens a strip of previews,
// the footer an editor — and both open in place, below their own row, so the
// pane never stops being one list.
export function AppearanceSettings() {
  const t = useT()
  const [themePendingRemoval, setThemePendingRemoval] = useState<{
    id: string
    name: string
  } | null>(null)
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
    brokenThemes,
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
      const path = await ProjectService.PickFile(t("settings.appearanceSettings.importPickTitle"))
      if (!path) return
      const result = await importTheme(path, false)
      setImportOpen(false)
      if (result.needsOverwrite) {
        setThemePendingOverwrite({ path, theme: result.theme })
        return
      }
      toast.success(t("settings.appearanceSettings.imported", { name: result.theme.name }))
    } catch (error) {
      toast.error(t("settings.appearanceSettings.importFailed", { error: errorText(error) }))
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
        t("settings.appearanceSettings.installed", {
          count: result.themes?.length ?? 0,
          pack: result.pack,
          version: result.version,
        }),
      )
    } catch (error) {
      toast.error(t("settings.appearanceSettings.installFailed", { error: errorText(error) }))
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
          ? t("settings.appearanceSettings.upToDate", {
              pack: result.pack,
              version: result.version,
            })
          : t("settings.appearanceSettings.updated", {
              pack: result.pack,
              version: result.version,
            }),
      )
    } catch (error) {
      toast.error(t("settings.appearanceSettings.updateFailed", { error: errorText(error) }))
    } finally {
      setUpdatingID(null)
    }
  }

  const onOverwriteTheme = async () => {
    if (!themePendingOverwrite) return
    try {
      const result = await importTheme(themePendingOverwrite.path, true)
      toast.success(t("settings.appearanceSettings.imported", { name: result.theme.name }))
      setThemePendingOverwrite(null)
    } catch (error) {
      toast.error(t("settings.appearanceSettings.importFailed", { error: errorText(error) }))
    }
  }

  const onRemoveTheme = async () => {
    if (!themePendingRemoval) {
      return
    }
    try {
      await removeTheme(themePendingRemoval.id)
      toast.success(t("settings.appearanceSettings.removed", { name: themePendingRemoval.name }))
      setThemePendingRemoval(null)
    } catch (error) {
      toast.error(t("settings.appearanceSettings.removeFailed", { error: errorText(error) }))
    }
  }

  const onDownloadThemeTemplate = async () => {
    try {
      const path = await ProjectService.PickSaveFile(
        t("settings.appearanceSettings.templatePickTitle"),
        THEME_TEMPLATE_FILENAME,
      )
      if (!path) return
      await Themes.SaveTemplate(path)
      toast.success(t("settings.appearanceSettings.templateSaved", { path }))
    } catch (error) {
      toast.error(t("settings.appearanceSettings.templateFailed", { error: errorText(error) }))
    }
  }

  return (
    <>
      <ThemePicker
        themes={themes}
        brokenThemes={brokenThemes}
        value={theme}
        resolved={resolvedTheme}
        onSelect={(id: Theme) => setTheme(id)}
        onImport={() => setImportOpen(true)}
        onUpdate={(item) => void onUpdateTheme(item)}
        onRemove={setThemePendingRemoval}
        onRemoveBroken={(item) => setThemePendingRemoval({ id: item.id, name: item.id })}
        updatingID={updatingID}
      />

      <SettingRow title={t("settings.appearanceSettings.zoomTitle")}>
        <Stepper
          value={zoom}
          display={`${Math.round(zoom * 100)}%`}
          min={ZOOM_MIN}
          max={ZOOM_MAX}
          step={ZOOM_STEP}
          fallback={DEFAULT_ZOOM}
          resetLabel={t("settings.appearanceSettings.resetZoom")}
          onChange={setZoom}
          decrementIcon={<ZoomOut />}
          incrementIcon={<ZoomIn />}
          decrementLabel={t("settings.appearanceSettings.zoomOut")}
          incrementLabel={t("settings.appearanceSettings.zoomIn")}
        />
      </SettingRow>

      <SettingRow title={t("settings.appearanceSettings.textSizeTitle")}>
        <Stepper
          value={terminalFontSize}
          display={`${terminalFontSize}px`}
          min={TERMINAL_FONT_SIZE_MIN}
          max={TERMINAL_FONT_SIZE_MAX}
          step={TERMINAL_FONT_SIZE_STEP}
          fallback={DEFAULT_TERMINAL_FONT_SIZE}
          resetLabel={t("settings.appearanceSettings.resetTextSize")}
          onChange={setTerminalFontSize}
          decrementIcon={<Minus />}
          incrementIcon={<Plus />}
          decrementLabel={t("settings.appearanceSettings.textSmaller")}
          incrementLabel={t("settings.appearanceSettings.textLarger")}
        />
      </SettingRow>

      <FontSetting />
      <FooterSettings />
      <LanguageSettings />

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
        title={t("settings.appearanceSettings.replaceThemesTitle")}
        description={
          <Trans
            k="settings.appearanceSettings.replaceThemesDescription"
            params={{
              themes: (
                <span className="font-medium">{packPendingOverwrite?.conflicts.join(", ")}</span>
              ),
            }}
          />
        }
      >
        <Button
          variant="destructive"
          disabled={importing}
          onClick={() => {
            if (packPendingOverwrite) void installRepository(packPendingOverwrite.url, true)
          }}
        >
          {t("settings.appearanceSettings.replaceThemes")}
        </Button>
      </ConfirmDialog>
      <ConfirmDialog
        open={themePendingRemoval !== null}
        onCancel={() => setThemePendingRemoval(null)}
        title={t("settings.appearanceSettings.removeThemeTitle")}
        description={
          <Trans
            k="settings.appearanceSettings.removeThemeDescription"
            params={{ name: <span className="font-medium">{themePendingRemoval?.name}</span> }}
          />
        }
      >
        <Button variant="destructive" onClick={() => void onRemoveTheme()}>
          {t("settings.appearanceSettings.removeTheme")}
        </Button>
      </ConfirmDialog>
      <ConfirmDialog
        open={themePendingOverwrite !== null}
        onCancel={() => setThemePendingOverwrite(null)}
        title={t("settings.appearanceSettings.replaceThemeTitle")}
        description={
          <Trans
            k="settings.appearanceSettings.replaceThemeDescription"
            params={{
              id: <span className="font-medium">{themePendingOverwrite?.theme.id}</span>,
            }}
          />
        }
      >
        <Button variant="destructive" onClick={() => void onOverwriteTheme()}>
          {t("settings.appearanceSettings.replaceTheme")}
        </Button>
      </ConfirmDialog>
    </>
  )
}

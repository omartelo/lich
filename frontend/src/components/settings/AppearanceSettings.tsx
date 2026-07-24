import {
  CaseSensitive,
  Minus,
  Monitor,
  Moon,
  Plus,
  RotateCcw,
  SquareTerminal,
  Sun,
  ZoomIn,
  ZoomOut,
} from "lucide-react"
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
} from "@/lib/settings"
import type { TerminalTheme, Theme } from "@/lib/settings"
import { Button } from "@/components/ui/button"
import { SegmentedControl } from "./SegmentedControl"
import { SettingBlock, SettingGroup } from "./SettingBlock"
import { FontSetting } from "./FontSetting"

const THEME_OPTIONS: ReadonlyArray<{ value: Theme; label: string; icon: JSX.Element }> = [
  { value: "system", label: "System", icon: <Monitor /> },
  { value: "light", label: "Light", icon: <Sun /> },
  { value: "dark", label: "Dark", icon: <Moon /> },
]

const TERMINAL_THEME_OPTIONS: ReadonlyArray<{
  value: TerminalTheme
  label: string
  icon: JSX.Element
}> = [
  { value: "match", label: "Match app", icon: <Monitor /> },
  { value: "light", label: "Light", icon: <Sun /> },
  { value: "dark", label: "Dark", icon: <Moon /> },
]

// Appearance holds every look-and-feel control, split into an Interface group
// (theme, zoom) and a Terminal group (background, text size, font) so the two
// concerns read apart instead of as one flat list. The group label supplies the
// context, so the block titles drop their "Interface"/"Terminal" prefix.
export function AppearanceSettings() {
  const {
    theme,
    setTheme,
    zoom,
    setZoom,
    terminalFontSize,
    setTerminalFontSize,
    terminalTheme,
    setTerminalTheme,
  } = useSettings()

  return (
    <>
      <SettingGroup label="Interface">
        <SettingBlock title="Theme">
          <SegmentedControl
            ariaLabel="Interface theme"
            value={theme}
            onChange={setTheme}
            options={THEME_OPTIONS}
          />
        </SettingBlock>

        <SettingBlock
          icon={<ZoomIn className="size-4" />}
          title="Zoom"
          description="Scales the interface."
        >
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              aria-label="Zoom out"
              disabled={zoom <= ZOOM_MIN}
              onClick={() => setZoom(zoom - ZOOM_STEP)}
            >
              <ZoomOut />
            </Button>
            <div className="flex h-9 min-w-16 items-center justify-center rounded-lg border border-border px-3 text-sm tabular-nums text-foreground">
              {Math.round(zoom * 100)}%
            </div>
            <Button
              variant="outline"
              size="icon"
              aria-label="Zoom in"
              disabled={zoom >= ZOOM_MAX}
              onClick={() => setZoom(zoom + ZOOM_STEP)}
            >
              <ZoomIn />
            </Button>
            <Button
              variant="ghost"
              disabled={zoom === DEFAULT_ZOOM}
              onClick={() => setZoom(DEFAULT_ZOOM)}
            >
              <RotateCcw />
              Reset
            </Button>
          </div>
        </SettingBlock>
      </SettingGroup>

      <SettingGroup label="Terminal">
        <SettingBlock
          icon={<SquareTerminal className="size-4" />}
          title="Appearance"
          description="Background color of the terminal. Match app keeps it in sync with the interface theme so text stays legible."
        >
          <SegmentedControl
            ariaLabel="Terminal appearance"
            value={terminalTheme}
            onChange={setTerminalTheme}
            options={TERMINAL_THEME_OPTIONS}
          />
        </SettingBlock>

        <SettingBlock
          icon={<CaseSensitive className="size-4" />}
          title="Text size"
          description="Scales the terminal."
        >
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              aria-label="Smaller terminal text"
              disabled={terminalFontSize <= TERMINAL_FONT_SIZE_MIN}
              onClick={() => setTerminalFontSize(terminalFontSize - TERMINAL_FONT_SIZE_STEP)}
            >
              <Minus />
            </Button>
            <div className="flex h-9 min-w-16 items-center justify-center rounded-lg border border-border px-3 text-sm tabular-nums text-foreground">
              {terminalFontSize}px
            </div>
            <Button
              variant="outline"
              size="icon"
              aria-label="Larger terminal text"
              disabled={terminalFontSize >= TERMINAL_FONT_SIZE_MAX}
              onClick={() => setTerminalFontSize(terminalFontSize + TERMINAL_FONT_SIZE_STEP)}
            >
              <Plus />
            </Button>
            <Button
              variant="ghost"
              disabled={terminalFontSize === DEFAULT_TERMINAL_FONT_SIZE}
              onClick={() => setTerminalFontSize(DEFAULT_TERMINAL_FONT_SIZE)}
            >
              <RotateCcw />
              Reset
            </Button>
          </div>
        </SettingBlock>

        <FontSetting />
      </SettingGroup>
    </>
  )
}

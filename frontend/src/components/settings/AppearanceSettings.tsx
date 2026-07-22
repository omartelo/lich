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
import { SettingBlock } from "./SettingBlock"

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
      <SettingBlock title="Interface theme">
        <SegmentedControl
          ariaLabel="Interface theme"
          value={theme}
          onChange={setTheme}
          options={THEME_OPTIONS}
        />
      </SettingBlock>

      <SettingBlock
        icon={<ZoomIn className="size-4" />}
        title="Interface zoom"
        description="Scales the app's interface — rail, tabs, sidebar, footer. The terminal is not affected: its text has its own size below, so zooming never rewraps a running session. You can also use Ctrl/Cmd + / − / 0 or Ctrl + mouse wheel, even inside a terminal."
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
          <div className="min-w-16 rounded-lg border border-border px-3 py-1.5 text-center text-sm tabular-nums text-foreground">
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

      <SettingBlock
        icon={<SquareTerminal className="size-4" />}
        title="Terminal appearance"
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
        title="Terminal text size"
        description="Size of the terminal's text, in pixels. Unlike interface zoom this does resize the grid, so a running session is told about the new width and height."
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
          <div className="min-w-16 rounded-lg border border-border px-3 py-1.5 text-center text-sm tabular-nums text-foreground">
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
    </>
  )
}

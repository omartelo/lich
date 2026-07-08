import { Moon, Monitor, RotateCcw, Sun, SquareTerminal, ZoomIn, ZoomOut } from "lucide-react"
import {
  DEFAULT_ZOOM,
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
        description="Increases or decreases the size of everything in the app (rail, tabs…). You can also use Ctrl + / − with focus outside the terminal."
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
    </>
  )
}

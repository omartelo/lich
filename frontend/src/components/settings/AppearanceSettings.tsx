import {
  CaseSensitive,
  Minus,
  Monitor,
  Moon,
  Plus,
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
  THEMES,
  TERMINAL_THEMES,
  ZOOM_STEP,
  useSettings,
} from "@/providers/settings"
import type { TerminalTheme, Theme } from "@/providers/settings"
import { Stepper } from "@/components/common/Stepper"
import { SegmentedControl } from "./SegmentedControl"
import { SettingBlock, SettingGroup } from "./SettingBlock"
import { FontSetting } from "./FontSetting"

// How each theme value presents itself. Built off THEMES/TERMINAL_THEMES so the
// picker cannot drift from what the provider persists — adding a theme there
// is what puts it on screen, and forgetting an entry here is a type error.
const THEME_LABELS: Record<Theme, { label: string; icon: JSX.Element }> = {
  system: { label: "System", icon: <Monitor /> },
  light: { label: "Light", icon: <Sun /> },
  dark: { label: "Dark", icon: <Moon /> },
}

const TERMINAL_THEME_LABELS: Record<TerminalTheme, { label: string; icon: JSX.Element }> = {
  match: { label: "Match app", icon: <Monitor /> },
  light: { label: "Light", icon: <Sun /> },
  dark: { label: "Dark", icon: <Moon /> },
}

const THEME_OPTIONS = THEMES.map((value) => ({ value, ...THEME_LABELS[value] }))

const TERMINAL_THEME_OPTIONS = TERMINAL_THEMES.map((value) => ({
  value,
  ...TERMINAL_THEME_LABELS[value],
}))

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

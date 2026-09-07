import { useState } from "react"
import { ChevronDown, RefreshCw, Trash2, Upload } from "lucide-react"
import type { ThemeDefinition } from "@/lib/api-types"
import { bundledThemes, repoLabel, SYSTEM_THEME } from "@/lib/themes"
import type { Theme } from "@/providers/settings"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { SettingRow } from "./SettingBlock"

// A theme is a set of colors, and a dropdown of names says nothing about them:
// the old select answered "Rosé Pine" and the only way to see it was to apply
// it. Every theme carries the tokens this draws, so the picker shows what it is
// choosing between.
interface ThemeMiniatureProps {
  theme: ThemeDefinition
  className?: string
  /** Drawn inside another miniature's frame, so it brings no edge of its own. */
  bare?: boolean
}

// The lich window at a glance: sidebar, canvas, and three lines standing for
// the text, the muted text and a filled row. Not a screenshot, but the same
// four tokens the real window leads with, in the same places.
export function ThemeMiniature({ theme, className, bare }: ThemeMiniatureProps) {
  return (
    <span
      aria-hidden
      className={cn(
        "grid shrink-0 grid-cols-[26%_1fr] overflow-hidden",
        // The edge is the foreground rather than the border token: a dark
        // theme's own background is the same value as the pane behind it, and
        // --border is white at 10% there, so the card would have no edge at all.
        !bare && "rounded-sm ring-1 ring-inset ring-foreground/20",
        className,
      )}
      style={{ backgroundColor: theme.app.background }}
    >
      <span style={{ backgroundColor: theme.app.sidebar }} />
      <span className="flex flex-col justify-start gap-[15%] p-[12%]">
        <span
          className="block h-[8%] min-h-px w-[60%] rounded-full"
          style={{ backgroundColor: theme.app.foreground }}
        />
        <span
          className="block h-[8%] min-h-px w-[85%] rounded-full"
          style={{ backgroundColor: theme.app["muted-foreground"] }}
        />
        <span
          className="block h-[8%] min-h-px w-[45%] rounded-full"
          style={{ backgroundColor: theme.app.accent }}
        />
      </span>
    </span>
  )
}

// The System card is the only one whose colors are not its own: it is whichever
// bundled theme the OS is asking for, so it shows both halves rather than
// picking one and lying about the other half of the time.
function SystemMiniature({
  themes,
  className,
}: {
  themes: readonly ThemeDefinition[]
  className?: string
}) {
  const [light, dark] = bundledThemes(themes)
  return (
    <span
      aria-hidden
      className={cn(
        "grid shrink-0 grid-cols-2 overflow-hidden rounded-sm ring-1 ring-inset ring-foreground/20",
        className,
      )}
    >
      {/* Both windows, cut down the middle. Two flat halves of background were
          the honest colors and still read as one empty card, because a dark
          theme's background is the same value as the pane it sits on. */}
      {light && <ThemeMiniature theme={light} bare className="h-full w-full" />}
      {dark && <ThemeMiniature theme={dark} bare className="h-full w-full" />}
    </span>
  )
}

interface ThemePickerProps {
  themes: readonly ThemeDefinition[]
  /** The selected id, which may be SYSTEM_THEME rather than a theme's own. */
  value: Theme
  /** The theme those colors resolve to, which is what the trigger shows. */
  resolved: ThemeDefinition
  onSelect: (id: Theme) => void
  onImport: () => void
  onUpdate: (theme: ThemeDefinition) => void
  onRemove: (theme: ThemeDefinition) => void
  updatingID: string | null
}

export function ThemePicker({
  themes,
  value,
  resolved,
  onSelect,
  onImport,
  onUpdate,
  onRemove,
  updatingID,
}: ThemePickerProps) {
  const [open, setOpen] = useState(false)
  const system = value === SYSTEM_THEME
  const choose = (id: Theme) => {
    onSelect(id)
    // The trigger repaints with the theme just picked, which is the receipt for
    // the click; leaving the strip open would hide it behind the list.
    setOpen(false)
  }

  return (
    <div className="min-w-0">
      <SettingRow title="Theme" description="Colors the interface and the terminal.">
        <Button
          type="button"
          variant="outline"
          aria-expanded={open}
          aria-label="Choose a theme"
          className="h-10 justify-between gap-3 pl-1.5"
          onClick={() => setOpen((wasOpen) => !wasOpen)}
        >
          <span className="flex items-center gap-2.5">
            {system ? (
              <SystemMiniature themes={themes} className="h-7 w-11" />
            ) : (
              <ThemeMiniature theme={resolved} className="h-7 w-11" />
            )}
            <span className="text-sm font-medium">{system ? "System" : resolved.name}</span>
          </span>
          <ChevronDown className={cn("transition-transform", open && "rotate-180")} />
        </Button>
      </SettingRow>

      {open && (
        <div className="min-w-0">
          <div className="mb-2 flex items-center justify-between gap-4">
            <span className="text-xs text-muted-foreground">{themeCount(themes.length + 1)}</span>
            <Button type="button" variant="ghost" size="xs" onClick={onImport}>
              <Upload />
              Import
            </Button>
          </div>
          {/* Two rows that grow sideways: a tenth theme costs width, which
              scrolls, instead of height, which pushes the rest of the pane off
              the screen. */}
          <div className="grid grid-flow-col grid-rows-2 auto-cols-[9.5rem] gap-3 overflow-x-auto px-0.5 pb-3">
            <ThemeCard
              name="System"
              caption="follows the OS"
              selected={system}
              onSelect={() => choose(SYSTEM_THEME)}
              preview={<SystemMiniature themes={themes} className="h-[88px] w-full" />}
            />
            {themes.map((theme) => (
              <ThemeCard
                key={theme.id}
                name={theme.name}
                caption={
                  value === theme.id
                    ? "in use"
                    : theme.source
                      ? `${repoLabel(theme.source.url)} v${theme.source.version}`
                      : "bundled"
                }
                selected={value === theme.id}
                onSelect={() => choose(theme.id)}
                preview={<ThemeMiniature theme={theme} className="h-[88px] w-full" />}
                actions={
                  theme.origin === "custom" && (
                    <>
                      {theme.source && (
                        <Tooltip>
                          <TooltipTrigger
                            render={
                              <Button
                                type="button"
                                variant="ghost"
                                size="icon-xs"
                                aria-label={`Update ${theme.name}`}
                                disabled={updatingID !== null}
                                onClick={(event) => {
                                  event.stopPropagation()
                                  onUpdate(theme)
                                }}
                              />
                            }
                          >
                            <RefreshCw className={updatingID === theme.id ? "animate-spin" : ""} />
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
                              size="icon-xs"
                              aria-label={`Remove ${theme.name}`}
                              onClick={(event) => {
                                event.stopPropagation()
                                onRemove(theme)
                              }}
                            />
                          }
                        >
                          <Trash2 />
                        </TooltipTrigger>
                        <TooltipContent>{`Remove ${theme.name}`}</TooltipContent>
                      </Tooltip>
                    </>
                  )
                }
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function themeCount(count: number): string {
  return count === 1 ? "1 theme" : `${count} themes`
}

interface ThemeCardProps {
  name: string
  caption: string
  selected: boolean
  preview: React.ReactNode
  onSelect: () => void
  actions?: React.ReactNode
}

function ThemeCard({ name, caption, selected, preview, onSelect, actions }: ThemeCardProps) {
  return (
    <div className="group relative w-[9.5rem] shrink-0">
      <button
        type="button"
        onClick={onSelect}
        aria-current={selected}
        className={cn(
          "block w-full rounded-md text-left outline-none",
          "focus-visible:ring-2 focus-visible:ring-ring",
        )}
      >
        <span
          className={cn(
            "block overflow-hidden rounded-md",
            selected && "ring-2 ring-ring ring-offset-2 ring-offset-background",
          )}
        >
          {preview}
        </span>
        <span className="mt-1.5 flex items-baseline gap-1.5">
          <span className="truncate text-xs font-medium text-foreground">{name}</span>
          <span className="truncate text-[11px] text-muted-foreground">{caption}</span>
        </span>
      </button>
      {/* Update and remove ride the card rather than a row of their own: they
          belong to one theme, and only an imported theme has them. */}
      {actions && (
        <div className="absolute right-1 top-1 hidden gap-0.5 rounded-md bg-popover/90 p-0.5 group-hover:flex group-focus-within:flex">
          {actions}
        </div>
      )}
    </div>
  )
}

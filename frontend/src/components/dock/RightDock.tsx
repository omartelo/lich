import { Code, FileDiff, Maximize2, Minimize2, X } from "lucide-react"
import { useState } from "react"
import { ErrorBoundary } from "@/components/common/ErrorBoundary"
import { IconAction } from "@/components/common/IconAction"
import { ResizeHandle } from "@/components/common/ResizeHandle"
import { CollapseAllAction, useDiffBulk } from "@/components/diff/diff-bulk"
import { ReviewPanel } from "@/components/diff/ReviewPanel"
import { DiffStat } from "@/components/DiffStat"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { usePanelWidth } from "@/lib/use-panel-width"
import { useActiveSession } from "@/lib/session/use-active-session"
import { useGitStatus } from "@/lib/git/use-git-status"
import { FilesPanel } from "./FilesPanel"

export type DockTab = "files" | "review"

// One dock width serves both tabs, so there is one key for it. Named here like
// every other `lich.*` pref rather than spelled at the call site.
const WIDTH_KEY = "lich.dock.width"

// Wide enough for a diff line at the 12px review font, capped short of eating
// the terminal whole.
const MIN_REM = 20
const MAX_REM = 60
const DEFAULT_REM = 28

// Ghost tabs, sized down to the dock's 40px header — not the stock filled pill.
const TAB_CLASS =
  "gap-1 rounded-md px-2 py-0.5 text-xs hover:bg-accent/50 data-active:bg-accent data-active:text-accent-foreground dark:data-active:border-transparent dark:data-active:bg-accent dark:data-active:text-accent-foreground"

interface RightDockProps {
  tab: DockTab
  onTab: (tab: DockTab) => void
  onClose: () => void
}

// RightDock is the review split at the terminal's right, shared by the Files
// and Review tabs. It owns the surrounding chrome — drag-resizable width
// (persisted), full-screen toggle, the tab bar and the close button — so the
// two panels stay pure bodies. One dock width serves both tabs: it is a single
// panel that swaps contents, not two panels competing for the edge.
export function RightDock({ tab, onTab, onClose }: RightDockProps) {
  const { path } = useActiveSession()
  const status = useGitStatus(path)
  const [fullscreen, setFullscreen] = useState(false)
  const [bulk, toggleAll] = useDiffBulk()
  const { width, handleProps } = usePanelWidth({
    storageKey: WIDTH_KEY,
    minRem: MIN_REM,
    maxRem: MAX_REM,
    defaultRem: DEFAULT_REM,
    edge: "left",
  })

  return (
    <aside
      aria-label={tab === "files" ? "File browser" : "Review changes"}
      className={
        fullscreen
          ? "absolute inset-0 z-20 flex flex-col bg-sidebar"
          : "relative flex shrink-0 flex-col border-l border-border bg-sidebar"
      }
      style={fullscreen ? undefined : { width: `${width}rem` }}
    >
      <div className="flex h-10 shrink-0 items-center gap-2 border-b border-border px-2">
        <Tabs value={tab} onValueChange={(value) => onTab(value as DockTab)} className={"h-8"}>
          <TabsList className="h-auto p-0.5 bg-transparent gap-1">
            <TabsTrigger value="files" className={TAB_CLASS}>
              <Code className="size-3.5" />
              Code
            </TabsTrigger>
            <TabsTrigger value="review" className={TAB_CLASS}>
              <FileDiff className="size-3.5" />
              Review
              {status && status.files > 0 && (
                <DiffStat added={status.added} deleted={status.deleted} />
              )}
            </TabsTrigger>
          </TabsList>
        </Tabs>
        <span className="ml-auto flex items-center gap-1">
          {tab === "review" && status && status.files > 0 && (
            <CollapseAllAction open={bulk.open} onToggle={toggleAll} />
          )}
          <IconAction
            label={fullscreen ? "Exit full screen" : "Full screen"}
            onClick={() => setFullscreen((v) => !v)}
          >
            {fullscreen ? <Minimize2 className="size-3.5" /> : <Maximize2 className="size-3.5" />}
          </IconAction>
          <Button
            variant="ghost"
            size="icon-xs"
            onClick={onClose}
            aria-label="Close panel"
            className="text-muted-foreground"
          >
            <X className="size-4" />
          </Button>
        </span>
      </div>
      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Below the header, so a panel that throws leaves the tab bar, the
            full-screen toggle and the close button reachable — and switching
            tab is itself a way out, which is what the reset key buys. */}
        <ErrorBoundary
          label={tab === "files" ? "The file tree" : "The review panel"}
          resetKey={tab}
        >
          {tab === "files" ? <FilesPanel /> : <ReviewPanel bulk={bulk} />}
        </ErrorBoundary>
      </div>
      {!fullscreen && <ResizeHandle edge="left" label="Resize panel" handleProps={handleProps} />}
    </aside>
  )
}

import {useState} from "react"
import {Code, FileDiff, Maximize2, Minimize2, X} from "lucide-react"
import {Button} from "@/components/ui/button"
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs"
import {DiffStat} from "@/components/DiffStat"
import {HeaderAction} from "@/components/diff/FileDiff"
import {ReviewPanel} from "@/components/diff/ReviewPanel"
import {useActiveSession} from "@/lib/useActiveSession"
import {useGitStatus} from "@/lib/useGitStatus"
import {usePanelWidth} from "@/lib/use-panel-width"
import {FilesPanel} from "./FilesPanel"

export type DockTab = "files" | "review"

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
export function RightDock({tab, onTab, onClose}: RightDockProps) {
  const {path} = useActiveSession()
  const status = useGitStatus(path)
  const [fullscreen, setFullscreen] = useState(false)
  const {width, handleProps} = usePanelWidth({
    storageKey: "lich.dock.width",
    minRem: 20,
    maxRem: 60,
    defaultRem: 28,
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
      style={fullscreen ? undefined : {width: `${width}rem`}}
    >
      <div className="flex h-10 shrink-0 items-center gap-2 border-b border-border px-2">
        <Tabs value={tab} onValueChange={(value) => onTab(value as DockTab)} className={"h-8"}>
          <TabsList className="h-auto p-0.5 bg-transparent gap-1">
            <TabsTrigger value="files" className="gap-1 px-2 py-0.5 text-xs hover:bg-muted">
              <Code className="size-3.5"/>
              Code
            </TabsTrigger>
            <TabsTrigger value="review" className="gap-1 px-2 py-0.5 text-xs hover:bg-muted">
              <FileDiff className="size-3.5"/>
              Review
              {status && status.files > 0 && (
                <DiffStat added={status.added} deleted={status.deleted}/>
              )}
            </TabsTrigger>
          </TabsList>
        </Tabs>
        <span className="ml-auto flex items-center gap-1">
          <HeaderAction
            label={fullscreen ? "Exit full screen" : "Full screen"}
            onClick={() => setFullscreen((v) => !v)}
          >
            {fullscreen ? (
              <Minimize2 className="size-3.5"/>
            ) : (
              <Maximize2 className="size-3.5"/>
            )}
          </HeaderAction>
          <Button
            variant="ghost"
            size="icon-xs"
            onClick={onClose}
            aria-label="Close panel"
            className="text-muted-foreground"
          >
            <X className="size-4"/>
          </Button>
        </span>
      </div>
      <div className="flex flex-1 flex-col overflow-hidden">
        {tab === "files" ? <FilesPanel/> : <ReviewPanel/>}
      </div>
      {!fullscreen && (
        <div
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize panel"
          {...handleProps}
          className="absolute left-0 top-0 h-full w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent"
        />
      )}
    </aside>
  )
}

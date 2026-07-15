import {useCallback, useEffect, useState} from "react"
import {Maximize2, Minimize2, X} from "lucide-react"
import {toast} from "sonner"
import {ProjectService, Terminal as TerminalService} from "@/lib/rpc"
import {Button} from "@/components/ui/button"
import {useActiveSession} from "@/lib/useActiveSession"
import {discardTargets, parseDiff, type DiffFile} from "@/lib/diff"
import {useGitStatus} from "@/lib/useGitStatus"
import {usePanelWidth} from "@/lib/use-panel-width"
import {errorText} from "@/lib/utils"
import {DiffStat} from "@/components/DiffStat"
import {DiscardDialog} from "./DiscardDialog"
import {FileDiff, HeaderAction} from "./FileDiff"

interface DiffPanelProps {
  onClose: () => void
}

// DiffPanel is the review split at the terminal's right: the active session's
// uncommitted diff, one collapsible file at a time. Context-menu actions write
// file/line references into the session's PTY, mirroring the footer's
// attach-file button. It follows the active session like the footer does: a
// worktree session reviews its checkout, not the project root.
export function DiffPanel({onClose}: DiffPanelProps) {
  const {projectId, sessionId, path} = useActiveSession()
  const status = useGitStatus(path)
  const [files, setFiles] = useState<DiffFile[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [pendingDiscard, setPendingDiscard] = useState<DiffFile | null>(null)
  const [fullscreen, setFullscreen] = useState(false)
  const {width, handleProps} = usePanelWidth({
    storageKey: "lich.diffpanel.width",
    minRem: 20,
    maxRem: 60,
    defaultRem: 28,
    edge: "left",
  })

  const refresh = useCallback(async () => {
    if (!path) {
      return
    }
    try {
      const text = await ProjectService.DiffText(path)
      setFiles(parseDiff(text))
      setFailed(false)
    } catch {
      setFiles([])
      setFailed(true)
    }
  }, [path])

  // The 3s git-status poll doubles as the invalidation signal: the diff text is
  // only re-fetched when the stats actually move, so selections and scroll
  // survive idle ticks.
  useEffect(() => {
    void refresh()
  }, [refresh, status?.files, status?.added, status?.deleted])

  if (!projectId) {
    return null
  }

  const inject = (text: string) => {
    if (sessionId) {
      void TerminalService.Write(sessionId, text)
    }
  }

  // Reverting a rename touches both paths (new removed, old restored); the
  // panel refreshes immediately instead of waiting for the next poll tick.
  const discard = async () => {
    const file = pendingDiscard
    setPendingDiscard(null)
    if (!file) {
      return
    }
    try {
      for (const rel of discardTargets(file)) {
        await ProjectService.DiscardFile(path, rel)
      }
    } catch (err: unknown) {
      toast.error(`Failed to discard changes: ${errorText(err)}`)
    }
    void refresh()
  }

  return (
    <aside
      aria-label="Review changes"
      className={
        fullscreen
          ? "absolute inset-0 z-20 flex flex-col bg-sidebar"
          : "relative flex shrink-0 flex-col border-l border-border bg-sidebar"
      }
      style={fullscreen ? undefined : {width: `${width}rem`}}
    >
      <div className="flex h-9 shrink-0 items-center gap-2 border-b border-border px-3 text-xs">
        <span className="font-medium uppercase tracking-wide text-muted-foreground">
          Review
        </span>
        {status && status.files > 0 && (
          <span className="flex items-center gap-1.5">
            <DiffStat added={status.added} deleted={status.deleted}/>
          </span>
        )}
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
            aria-label="Close review panel"
            className="text-muted-foreground"
          >
            <X className="size-4"/>
          </Button>
        </span>
      </div>
      <div className="flex-1 overflow-y-auto">
        <PanelBody
          files={files}
          failed={failed}
          onInject={inject}
          onDiscard={setPendingDiscard}
        />
      </div>
      <DiscardDialog
        file={pendingDiscard}
        onCancel={() => setPendingDiscard(null)}
        onDiscard={() => void discard()}
      />
      {!fullscreen && (
        <div
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize review panel"
          {...handleProps}
          className="absolute left-0 top-0 h-full w-1.5 cursor-col-resize touch-none transition-colors hover:bg-accent"
        />
      )}
    </aside>
  )
}

interface PanelBodyProps {
  files: DiffFile[] | null
  failed: boolean
  onInject: (text: string) => void
  onDiscard: (file: DiffFile) => void
}

function PanelBody({files, failed, onInject, onDiscard}: PanelBodyProps) {
  if (failed) {
    return <PanelNotice>Not a git repository</PanelNotice>
  }
  if (files === null) {
    return <PanelNotice>Loading…</PanelNotice>
  }
  if (files.length === 0) {
    return <PanelNotice>No uncommitted changes</PanelNotice>
  }
  return (
    <div className="flex flex-col gap-2 p-2">
      {files.map((file) => (
        <FileDiff
          key={file.newPath}
          file={file}
          onInject={onInject}
          onDiscard={() => onDiscard(file)}
        />
      ))}
    </div>
  )
}

function PanelNotice({children}: { children: string }) {
  return <p className="px-3 py-4 text-xs text-muted-foreground">{children}</p>
}

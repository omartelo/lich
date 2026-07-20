import {useCallback, useEffect, useState} from "react"
import {toast} from "sonner"
import {ProjectService, Terminal as TerminalService} from "@/lib/rpc"
import {useActiveSession} from "@/lib/useActiveSession"
import {discardTargets, parseDiff, type DiffFile} from "@/lib/diff"
import {useGitStatus} from "@/lib/useGitStatus"
import {errorText} from "@/lib/utils"
import {DiscardDialog} from "./DiscardDialog"
import {FileDiff} from "./FileDiff"

// ReviewPanel is the Review tab's body: the active session's uncommitted diff,
// one collapsible file at a time. Context-menu actions write file/line
// references into the session's PTY, mirroring the footer's attach-file button.
// It follows the active session — a worktree session reviews its checkout, not
// the project root. The dock (RightDock) owns the surrounding chrome: width,
// full screen, the tab bar and the close button.
export function ReviewPanel() {
  const {sessionId, path} = useActiveSession()
  const status = useGitStatus(path)
  const [files, setFiles] = useState<DiffFile[] | null>(null)
  const [failed, setFailed] = useState(false)
  const [pendingDiscard, setPendingDiscard] = useState<DiffFile | null>(null)

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
    <div className="h-full overflow-y-auto">
      <PanelBody
        files={files}
        failed={failed}
        onInject={inject}
        onDiscard={setPendingDiscard}
      />
      <DiscardDialog
        file={pendingDiscard}
        onCancel={() => setPendingDiscard(null)}
        onDiscard={() => void discard()}
      />
    </div>
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

function PanelNotice({children}: {children: string}) {
  return <p className="px-3 py-4 text-xs text-muted-foreground">{children}</p>
}

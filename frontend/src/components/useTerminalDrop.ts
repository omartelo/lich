import { useRef, useState } from "react"
import type { DragEvent } from "react"
import { toast } from "sonner"
import { isWindows } from "@/lib/platform"
import {
  composeDroppedPaths,
  type DropTarget,
  readDroppedFiles,
  resolveDroppedFiles,
} from "@/lib/terminal/drop-files"

// carriesFiles tells a file drag from text dragged out of the app's own UI —
// a selection, a link — which the terminal leaves alone.
function carriesFiles(transfer: DataTransfer): boolean {
  return [...transfer.types].includes("Files")
}

export interface TerminalDrop {
  /** A file drag is over the terminal: draw the hint. */
  dropping: boolean
  // The four handlers the container wires, written onto it one by one rather
  // than spread: a11y lint reads JSX attributes, and a spread would hide the
  // static-element-interaction the container answers for in its own comment.
  onDrop: (event: DragEvent<HTMLDivElement>) => void
  onDragEnter: (event: DragEvent<HTMLDivElement>) => void
  onDragOver: (event: DragEvent<HTMLDivElement>) => void
  onDragLeave: () => void
}

// useTerminalDrop lands files dropped on a terminal at its prompt as paths, the
// way a native emulator pastes a drop (lib/terminal/drop-files.ts). `target` is
// the session the terminal belongs to — the tree the backend looks the files up
// in, and whether it is confined — and `write` is how the paths reach the PTY.
export function useTerminalDrop(
  target: DropTarget,
  write: (data: string) => void,
  focus: () => void,
): TerminalDrop {
  const [dropping, setDropping] = useState(false)
  // dragDepth counts enter/leave: they also fire crossing xterm's own child
  // elements, so the hint would flicker off mid-drag on the plain boolean.
  const dragDepth = useRef(0)

  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault()
    dragDepth.current = 0
    setDropping(false)
    const dropped = readDroppedFiles(event.dataTransfer)
    if (dropped.length === 0) {
      return
    }
    void (async () => {
      const { paths, skipped, notices } = await resolveDroppedFiles(target, dropped)
      // The notices ride in the paste rather than in a toast: a copy's path is
      // read by the agent, which never sees one, and a toast is gone by the
      // time anyone acts on the path.
      const paste = composeDroppedPaths(paths, isWindows, notices)
      if (paste !== "") {
        write(paste)
        focus()
      }
      if (skipped.length > 0) {
        toast.error(`Not attached: ${skipped.join(", ")}`)
      }
    })()
  }

  const onDragEnter = (event: DragEvent<HTMLDivElement>) => {
    if (!carriesFiles(event.dataTransfer)) {
      return
    }
    dragDepth.current += 1
    setDropping(true)
  }

  const onDragOver = (event: DragEvent<HTMLDivElement>) => {
    if (!carriesFiles(event.dataTransfer)) {
      return
    }
    // Without this the drop event never fires — and App's window handler
    // swallows what lands outside a terminal.
    event.preventDefault()
    event.dataTransfer.dropEffect = "copy"
  }

  const onDragLeave = () => {
    dragDepth.current = Math.max(0, dragDepth.current - 1)
    if (dragDepth.current === 0) {
      setDropping(false)
    }
  }

  return { dropping, onDrop, onDragEnter, onDragOver, onDragLeave }
}

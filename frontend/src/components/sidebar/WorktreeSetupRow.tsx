import { useEffect, useState } from "react"
import { FileText, Terminal } from "lucide-react"
import { ProjectService } from "@/lib/rpc"
import type { WorktreeSetup } from "@/lib/api-types"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { errorText } from "@/lib/utils"

// WorktreeSetupRow is the New-worktree dialog's pre-flight line: the script a
// fresh worktree will run (.lich/setup-worktree.sh, read from the main
// checkout), or a lockfile-detected suggestion when the repository ships none.
// Use/Save write the file through the backend; the row never blocks Create —
// informative when configured, one accept away when not, absent when there is
// nothing to say.
export function WorktreeSetupRow({ projectPath }: { projectPath: string }) {
  const [info, setInfo] = useState<WorktreeSetup | null>(null)
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState("")
  const [error, setError] = useState("")

  useEffect(() => {
    let stale = false
    // A failed read keeps the row absent: the dialog's job is the branch pick,
    // and a setup lich cannot read is a setup it will not run either.
    ProjectService.WorktreeSetup(projectPath)
      .then((loaded) => {
        if (!stale) {
          setInfo(loaded)
        }
      })
      .catch(() => {})
    return () => {
      stale = true
    }
  }, [projectPath])

  const save = async (script: string) => {
    setError("")
    try {
      await ProjectService.SaveWorktreeSetup(projectPath, script)
      setEditing(false)
      setInfo(await ProjectService.WorktreeSetup(projectPath))
    } catch (err) {
      setError(errorText(err))
    }
  }

  if (!info) {
    return null
  }

  if (editing) {
    return (
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="worktree-setup" className="text-xs uppercase tracking-wide">
          Setup script — saved to .lich/setup-worktree.sh
        </Label>
        <textarea
          id="worktree-setup"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
          rows={3}
          ref={(element) => element?.focus()}
          spellCheck={false}
          placeholder="pnpm install"
          className="w-full resize-none rounded-md border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
        />
        <div className="flex items-center justify-between">
          <span className="font-mono text-xs text-muted-foreground">
            $LICH_WORKTREE_PORT · $LICH_PROJECT_DIR
          </span>
          <div className="flex gap-1">
            <Button variant="ghost" size="xs" onClick={() => setEditing(false)}>
              Discard
            </Button>
            <Button variant="ghost" size="xs" onClick={() => void save(draft)}>
              Save
            </Button>
          </div>
        </div>
        {error && <span className="text-xs break-words text-destructive">{error}</span>}
      </div>
    )
  }

  if (info.script) {
    const [first, ...rest] = info.script.split("\n")
    return (
      <div className="flex min-w-0 items-center gap-2 text-xs">
        <Terminal className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
        <span className="min-w-0 truncate font-mono">
          {first}
          {rest.length > 0 && " …"}
        </span>
        <span className="ml-auto flex shrink-0 items-center gap-1 text-muted-foreground">
          <FileText className="size-3" aria-hidden />
          <span className="font-mono">.lich/setup-worktree.sh</span>
        </span>
      </div>
    )
  }

  if (!info.suggestion) {
    return null
  }
  return (
    <div className="flex flex-col gap-1">
      <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
        <Terminal className="size-3.5 shrink-0" aria-hidden />
        <span className="min-w-0 truncate">
          Detected {info.detected} — run{" "}
          <span className="font-mono text-foreground">{info.suggestion}</span> in new worktrees?
        </span>
        <div className="ml-auto flex shrink-0 gap-1">
          <Button variant="ghost" size="xs" onClick={() => void save(info.suggestion ?? "")}>
            Use
          </Button>
          <Button
            variant="ghost"
            size="xs"
            onClick={() => {
              setDraft(info.suggestion ?? "")
              setEditing(true)
            }}
          >
            Edit
          </Button>
        </div>
      </div>
      {error && <span className="text-xs break-words text-destructive">{error}</span>}
    </div>
  )
}

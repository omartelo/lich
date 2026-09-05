import { useEffect, useState } from "react"
import { FileText, Play, Terminal } from "lucide-react"
import { ProjectService } from "@/lib/rpc"
import type { WorktreeSetup } from "@/lib/api-types"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { errorText } from "@/lib/utils"

// Which of the two scripts a row is about. They are the same shape — one file
// in .lich/, read from the main checkout, edited in the same textarea — and
// differ only in what they run and when.
type Script = "setup" | "run"

const files: Record<Script, string> = {
  setup: ".lich/setup-worktree.sh",
  run: ".lich/run-worktree.sh",
}

const placeholders: Record<Script, string> = {
  setup: "pnpm install",
  run: "pnpm dev --port $LICH_WORKTREE_PORT",
}

// ScriptEditor is the textarea both rows open into, with the two variables a
// script can count on named under it.
function ScriptEditor({
  script,
  draft,
  onDraft,
  onCancel,
  onSave,
}: {
  script: Script
  draft: string
  onDraft: (value: string) => void
  onCancel: () => void
  onSave: () => void
}) {
  const id = `worktree-${script}`
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id} className="text-xs uppercase tracking-wide">
        {script === "setup" ? "Setup script" : "Run command"} — saved to {files[script]}
      </Label>
      <textarea
        id={id}
        value={draft}
        onChange={(event) => onDraft(event.target.value)}
        rows={3}
        ref={(element) => element?.focus()}
        spellCheck={false}
        placeholder={placeholders[script]}
        className="w-full resize-none rounded-md border border-input bg-transparent px-2.5 py-1.5 font-mono text-xs outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
      />
      <div className="flex items-center justify-between">
        <span className="font-mono text-xs text-muted-foreground">
          $LICH_WORKTREE_PORT · $LICH_PROJECT_DIR
        </span>
        <div className="flex gap-1">
          <Button variant="ghost" size="xs" onClick={onCancel}>
            Discard
          </Button>
          <Button variant="ghost" size="xs" onClick={onSave}>
            Save
          </Button>
        </div>
      </div>
    </div>
  )
}

// ConfiguredRow is a script the repository ships: its first line, and the file
// it lives in.
function ConfiguredRow({ script, value }: { script: Script; value: string }) {
  const [first, ...rest] = value.split("\n")
  const Icon = script === "setup" ? Terminal : Play
  return (
    <div className="flex min-w-0 items-center gap-2 text-xs">
      <Icon className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
      <span className="min-w-0 truncate font-mono">
        {first}
        {rest.length > 0 && " …"}
      </span>
      <span className="ml-auto flex shrink-0 items-center gap-1 text-muted-foreground">
        <FileText className="size-3" aria-hidden />
        <span className="font-mono">{files[script]}</span>
      </span>
    </div>
  )
}

// WorktreeScriptRows is the New-worktree dialog's pre-flight block: the setup
// script a fresh worktree will run (or a lockfile-detected suggestion when the
// repository ships none), and the run command every checkout of the project
// opens a Run card on. Both are read from the main checkout; Use/Save write the
// file through the backend. Neither ever blocks Create.
export function WorktreeScriptRows({ projectPath }: { projectPath: string }) {
  const [info, setInfo] = useState<WorktreeSetup | null>(null)
  const [editing, setEditing] = useState<Script | null>(null)
  const [draft, setDraft] = useState("")
  const [error, setError] = useState("")

  useEffect(() => {
    let stale = false
    // A failed read keeps the block absent: the dialog's job is the branch pick,
    // and a script lich cannot read is a script it will not run either.
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

  const save = async (script: Script, value: string) => {
    setError("")
    try {
      if (script === "setup") {
        await ProjectService.SaveWorktreeSetup(projectPath, value)
      } else {
        await ProjectService.SaveWorktreeRun(projectPath, value)
      }
      setEditing(null)
      setInfo(await ProjectService.WorktreeSetup(projectPath))
    } catch (err) {
      setError(errorText(err))
    }
  }

  const edit = (script: Script, value: string) => {
    setDraft(value)
    setEditing(script)
  }

  if (!info) {
    return null
  }

  if (editing) {
    return (
      <div className="flex flex-col gap-1">
        <ScriptEditor
          script={editing}
          draft={draft}
          onDraft={setDraft}
          onCancel={() => setEditing(null)}
          onSave={() => void save(editing, draft)}
        />
        {error && <span className="text-xs break-words text-destructive">{error}</span>}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-1.5">
      {info.script ? (
        <ConfiguredRow script="setup" value={info.script} />
      ) : (
        info.suggestion && (
          <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
            <Terminal className="size-3.5 shrink-0" aria-hidden />
            <span className="min-w-0 truncate">
              Detected {info.detected} — run{" "}
              <span className="font-mono text-foreground">{info.suggestion}</span> in new worktrees?
            </span>
            <div className="ml-auto flex shrink-0 gap-1">
              <Button
                variant="ghost"
                size="xs"
                onClick={() => void save("setup", info.suggestion ?? "")}
              >
                Use
              </Button>
              <Button
                variant="ghost"
                size="xs"
                onClick={() => edit("setup", info.suggestion ?? "")}
              >
                Edit
              </Button>
            </div>
          </div>
        )
      )}
      {info.run ? (
        <ConfiguredRow script="run" value={info.run} />
      ) : (
        // Offered rather than detected: a lockfile names how a checkout
        // installs, never how the project starts. Always on screen, because a
        // row nobody can see is a feature nobody finds.
        <div className="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
          <Play className="size-3.5 shrink-0" aria-hidden />
          <span className="min-w-0 truncate">
            No run command — the checkout&apos;s reserved port stays unused.
          </span>
          <Button
            variant="ghost"
            size="xs"
            className="ml-auto shrink-0"
            onClick={() => edit("run", "")}
          >
            Set
          </Button>
        </div>
      )}
      {error && <span className="text-xs break-words text-destructive">{error}</span>}
    </div>
  )
}

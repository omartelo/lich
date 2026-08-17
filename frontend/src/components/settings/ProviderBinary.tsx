import { useEffect, useState, type ReactNode } from "react"
import { Check, ChevronDown, FolderOpen, TriangleAlert, X } from "lucide-react"
import { toast } from "sonner"
import type { BinaryCheck } from "@/lib/api-types"
import { checkDetail, checkLabel, failed, winningScope } from "@/lib/binary-layers"
import { ProjectService, Store } from "@/lib/rpc"
import { useBinaryCheck } from "@/lib/use-binary-check"
import { binKey } from "@/lib/providers-store"
import { useProjects } from "@/providers/projects"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { SettingBlock } from "./SettingBlock"

const GLOBAL_SCOPE = ""

// ProviderBinary is the "which executable runs" block. Closed it is a fact — the
// binary a session here would spawn, and whether it is there — because almost
// nobody sets this and the question they do have is that one. Opening it shows
// the resolution as it is: the project's override, the global one and $PATH, in
// the order the spawn reads them, with the winning layer marked. The precedence
// stops being a sentence to trust and becomes the thing on screen.
//
// A layer that resolves to nothing runnable opens the block itself: an error
// hidden behind a disclosure is an error nobody fixes.
export function ProviderBinary({
  providerId,
  providerName,
  projectId,
}: {
  providerId: string
  providerName: string
  projectId?: string
}) {
  const { projects } = useProjects()
  const project = projects.find((p) => p.id === projectId)
  const key = binKey(providerId)
  const [globalBin, setGlobalBin] = useState("")
  const [projectBin, setProjectBin] = useState("")
  const [open, setOpen] = useState(false)

  useEffect(() => {
    void Store.GetSetting(key, GLOBAL_SCOPE).then(setGlobalBin)
  }, [key])

  useEffect(() => {
    if (!projectId) {
      return
    }
    void Store.GetSetting(key, projectId).then(setProjectBin)
  }, [key, projectId])

  const persistGlobal = (value: string) => {
    setGlobalBin(value)
    void Store.SetSetting(key, GLOBAL_SCOPE, value.trim())
  }

  const persistProject = (value: string) => {
    setProjectBin(value)
    if (projectId) {
      void Store.SetSetting(key, projectId, value.trim())
    }
  }

  const scope = winningScope(project ? projectBin : "", globalBin)
  const configured = scope === "project" ? projectBin.trim() : globalBin.trim()
  // Both layers go through the same check, so the bottom row answers with the
  // resolution the spawn would make rather than a second opinion about $PATH.
  const typed = useBinaryCheck(scope === "path" ? "" : configured)
  const onPath = useBinaryCheck(providerId)
  const check = scope === "path" ? onPath : typed
  const broken = failed(check)

  const browse = async (persist: (value: string) => void) => {
    try {
      const file = await ProjectService.PickFile(`Choose the ${providerName} binary`)
      if (file) {
        persist(file)
      }
    } catch {
      toast.error("Could not open the file picker")
    }
  }

  const layers = [
    project && {
      scope: "project" as const,
      label: `${project.name} only`,
      value: projectBin,
      persist: persistProject,
    },
    {
      scope: "global" as const,
      label: "All projects",
      value: globalBin,
      persist: persistGlobal,
    },
  ].filter((layer) => layer !== undefined)

  return (
    <SettingBlock
      title="Binary"
      description={
        project
          ? `Which executable a session in ${project.name} spawns.`
          : "Which executable a session spawns."
      }
    >
      {broken || open ? (
        <>
          <div className="mb-3 flex items-center justify-between gap-4">
            <span className="flex items-center gap-1.5 text-xs text-foreground">
              <ChevronDown className="size-3.5" aria-hidden="true" />
              Where it comes from — the first layer with a path wins
            </span>
            {!broken && (
              <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>
                Done
              </Button>
            )}
          </div>
          <div className="flex flex-col divide-y divide-border">
            {layers.map((layer) => (
              <Layer
                key={layer.scope}
                label={layer.label}
                wins={scope === layer.scope}
                check={scope === layer.scope ? check : null}
                value={layer.value}
              >
                <Input
                  value={layer.value}
                  onChange={(event) => layer.persist(event.target.value)}
                  placeholder={providerId}
                  spellCheck={false}
                  aria-label={`${providerName} binary for ${layer.label}`}
                  className="h-8 min-w-0 flex-1 font-mono text-xs"
                />
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => void browse(layer.persist)}
                  aria-label={`Pick the ${providerName} binary for ${layer.label}`}
                >
                  <FolderOpen className="size-3.5" aria-hidden="true" />
                  Browse
                </Button>
              </Layer>
            ))}
            <Layer label="$PATH" wins={scope === "path"} check={onPath} value={providerId}>
              <span className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">
                {onPath?.path || providerId}
              </span>
            </Layer>
          </div>
        </>
      ) : (
        <div className="flex items-center justify-between gap-4">
          <span className="flex min-w-0 items-center gap-2 text-xs">
            <Verdict check={check} bin={configured || providerId} />
            <span className="truncate font-mono text-foreground">
              {check?.path || configured || providerId}
            </span>
            <span className="whitespace-nowrap text-muted-foreground">
              · {sourceLabel(scope, project?.name)}
            </span>
          </span>
          <Button variant="ghost" size="sm" onClick={() => setOpen(true)}>
            Use a different binary
          </Button>
        </div>
      )}

      {broken && (
        <p className="mt-3 flex max-w-prose items-start gap-2 text-xs text-destructive">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
          {checkDetail(check)}
        </p>
      )}
    </SettingBlock>
  )
}

// sourceLabel names where the winning path came from, in the closed state where
// the layers themselves are not on screen.
function sourceLabel(scope: string, projectName: string | undefined): string {
  if (scope === "project") {
    return projectName ? `set for ${projectName}` : "set for this project"
  }
  return scope === "global" ? "set for all projects" : "from $PATH"
}

// Layer is one row of the resolution stack: the marker, the scope it configures,
// its control, and — on the winning row alone — what that path resolves to. The
// losing rows say they are overridden instead, which is the question a second
// path on screen raises.
function Layer({
  label,
  wins,
  check,
  value,
  children,
}: {
  label: string
  wins: boolean
  check: BinaryCheck | null
  value: string
  children: ReactNode
}) {
  return (
    <div className="flex items-center gap-2.5 py-2">
      <span
        className={cn(
          "size-1.5 shrink-0 rounded-full bg-muted-foreground opacity-30",
          wins && "opacity-100 bg-foreground",
          wins && failed(check) && "bg-destructive",
        )}
        aria-hidden="true"
      />
      <span
        className={cn("w-28 shrink-0 text-xs text-muted-foreground", wins && "text-foreground")}
      >
        {label}
      </span>
      {children}
      {wins ? (
        <span className="flex items-center gap-1 whitespace-nowrap text-xs">
          <Verdict check={check} bin={value} />
          <span className={cn(failed(check) ? "text-destructive" : "text-muted-foreground")}>
            {checkLabel(check, value)}
          </span>
        </span>
      ) : (
        <span className="whitespace-nowrap text-xs text-muted-foreground opacity-70">
          overridden
        </span>
      )}
    </div>
  )
}

// Verdict is the glyph beside a path. Nothing is drawn while a check is in
// flight, so a field being typed into does not blink between states.
function Verdict({ check, bin }: { check: BinaryCheck | null; bin: string }) {
  if (!check) {
    return null
  }
  const label = checkLabel(check, bin)
  if (failed(check)) {
    return <X className="size-3.5 shrink-0 text-destructive" aria-label={label} />
  }
  if (check.status === "ok") {
    return <Check className="size-3.5 shrink-0 text-emerald-500" aria-label={label} />
  }
  return null
}

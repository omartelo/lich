import { useEffect, useState } from "react"
import { Folder, FolderOpen, FolderX, Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { RecentProject } from "@/lib/api-types"
import { displayPath } from "@/lib/paths"
import { ProjectService, Store } from "@/lib/rpc"
import { useProjects } from "@/providers/projects"

// MENU_LIMIT is how many closed projects the menu offers. Five is what fits
// above the picker entry without turning the menu into a second project list;
// the store returns a longer list than this and the command palette searches
// the rest of it.
const MENU_LIMIT = 5

// OpenProjectMenu is the top strip's "+": the projects closed earlier, newest
// first, above the directory picker that opens any other one. With no closed
// project to offer there is no menu at all — the button is the picker, which is
// what it was before this list existed.
export function OpenProjectMenu() {
  const { projects, openProject, openRecent } = useProjects()
  const [recents, setRecents] = useState<RecentProject[]>([])
  const [missing, setMissing] = useState<ReadonlySet<string>>(new Set())

  // The open projects are exactly what the recent ones are not, so the list is
  // refetched whenever they change: closing a tab adds one, opening removes it.
  // The directories are checked with it, so an entry that moved says what
  // clicking it will ask for instead of opening a picker out of nowhere.
  useEffect(() => {
    let live = true
    void Store.RecentProjects().then((rows) => {
      const shown = (rows ?? []).slice(0, MENU_LIMIT)
      if (!live) {
        return
      }
      setRecents(shown)
      // Asked for after the list is up: the mark is what a row says about
      // itself, and a failed check must not cost the menu its entries.
      void ProjectService.Missing(shown.map((row) => row.path)).then((gone) => {
        if (live) {
          setMissing(new Set(gone ?? []))
        }
      })
    })
    return () => {
      live = false
    }
  }, [projects])

  if (recents.length === 0) {
    return (
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => void openProject()}
        title="Open project"
        aria-label="Open project"
        className="text-muted-foreground"
      >
        <Plus className="size-4" />
      </Button>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        title="Open project"
        aria-label="Open project"
        render={
          <Button variant="ghost" size="icon-sm" className="shrink-0 text-muted-foreground" />
        }
      >
        <Plus className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-72">
        {/* A plain div, not DropdownMenuLabel: that is base-ui's Menu.GroupLabel
            and throws outside a Menu.Group. */}
        <div className="px-2 py-1.5 text-xs font-semibold tracking-wide text-foreground">
          Recent projects
        </div>
        <DropdownMenuSeparator />
        {recents.map((recent) => {
          const gone = missing.has(recent.path)
          const Icon = gone ? FolderX : Folder
          return (
            <DropdownMenuItem
              key={recent.id}
              className="gap-2"
              onClick={() => void openRecent(recent)}
            >
              <Icon className="size-4 shrink-0 text-muted-foreground" />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate">{recent.name}</span>
                <span className="truncate text-xs text-muted-foreground">
                  {displayPath(recent.path)}
                </span>
              </span>
              {gone && (
                <span className="shrink-0 self-center font-mono text-[0.625rem] text-muted-foreground">
                  relocate
                </span>
              )}
            </DropdownMenuItem>
          )
        })}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="gap-2" onClick={() => void openProject()}>
          <FolderOpen className="size-4 shrink-0 text-muted-foreground" />
          Open folder…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

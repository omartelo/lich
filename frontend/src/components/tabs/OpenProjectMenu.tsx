import { useEffect, useState } from "react"
import { Folder, FolderOpen, Plus } from "lucide-react"

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
import { Store } from "@/lib/rpc"
import { useProjects } from "@/providers/projects"

// OpenProjectMenu is the top strip's "+": the projects closed earlier, newest
// first, above the directory picker that opens any other one. With no closed
// project to offer there is no menu at all — the button is the picker, which is
// what it was before this list existed.
export function OpenProjectMenu() {
  const { projects, openProject, openRecent } = useProjects()
  const [recents, setRecents] = useState<RecentProject[]>([])

  // The open projects are exactly what the recent ones are not, so the list is
  // refetched whenever they change: closing a tab adds one, opening removes it.
  useEffect(() => {
    let live = true
    void Store.RecentProjects().then((rows) => {
      if (live) {
        setRecents(rows ?? [])
      }
    })
    return () => {
      live = false
    }
  }, [projects])

  // Picking an entry whose directory is gone drops its row without opening
  // anything, so the open projects — and the effect above — never move. The
  // list has to be asked again, or the dead entry stays on offer.
  const pick = async (recent: RecentProject) => {
    await openRecent(recent)
    setRecents((await Store.RecentProjects()) ?? [])
  }

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
        {recents.map((recent) => (
          <DropdownMenuItem
            key={recent.id}
            className="gap-2"
            onClick={() => void pick(recent)}
          >
            <Folder className="size-4 shrink-0 text-muted-foreground" />
            <span className="flex min-w-0 flex-col">
              <span className="truncate">{recent.name}</span>
              <span className="truncate text-xs text-muted-foreground">
                {displayPath(recent.path)}
              </span>
            </span>
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem className="gap-2" onClick={() => void openProject()}>
          <FolderOpen className="size-4 shrink-0 text-muted-foreground" />
          Open folder…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

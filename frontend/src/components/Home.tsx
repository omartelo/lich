import { FolderOpen } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/lib/projects"

// Home is the landing screen shown when no project is active. It sits on top of
// the (currently empty) terminal host and offers the OS directory picker.
export function Home() {
  const { openProject } = useProjects()

  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-4 bg-background">
      <FolderOpen className="size-12 text-muted-foreground" />
      <div className="text-center">
        <h1 className="text-lg font-semibold text-foreground">No project open</h1>
        <p className="text-sm text-muted-foreground">
          Open a folder to start a terminal session.
        </p>
      </div>
      <Button onClick={() => void openProject()}>
        <FolderOpen data-icon="inline-start" />
        Open project
      </Button>
    </div>
  )
}

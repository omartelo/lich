import { FolderOpen } from "lucide-react"
import { EmptyScreen } from "@/components/common/EmptyScreen"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/providers/projects"

// Home is the landing screen shown when no project is active. It sits on top of
// the (currently empty) terminal host and offers the OS directory picker.
export function Home() {
  const { openProject } = useProjects()

  return (
    <EmptyScreen
      icon={FolderOpen}
      title="No project open"
      description="Open a folder to start a terminal session."
    >
      <Button onClick={() => void openProject()}>
        <FolderOpen data-icon="inline-start" />
        Open project
      </Button>
    </EmptyScreen>
  )
}

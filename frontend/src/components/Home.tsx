import { FolderOpen } from "lucide-react"
import { EmptyScreen } from "@/components/common/EmptyScreen"
import { Button } from "@/components/ui/button"
import { useProjects } from "@/providers/projects"
import { useT } from "@/lib/i18n/i18n"

// Home is the landing screen shown when no project is active. It sits on top of
// the (currently empty) terminal host and offers the OS directory picker.
export function Home() {
  const t = useT()
  const { openProject } = useProjects()

  return (
    <EmptyScreen
      icon={FolderOpen}
      title={t("shell.home.title")}
      description={t("shell.home.description")}
    >
      <Button onClick={() => void openProject()}>
        <FolderOpen data-icon="inline-start" />
        {t("shell.home.openProject")}
      </Button>
    </EmptyScreen>
  )
}

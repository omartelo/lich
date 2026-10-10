import { Settings } from "lucide-react"
import { SidebarCard } from "@/components/common/SidebarCard"
import { useT } from "@/lib/i18n/i18n"

interface SettingsCardProps {
  active: boolean
  onSelect: () => void
  onClose: () => void
}

// SettingsCard is the project's Settings entry in the session list: it appears
// when settings is opened for the project and stays parked (inactive) while the
// user works in a terminal, mirroring SessionCard's shape so it reads as a peer
// of the sessions rather than a separate control.
export function SettingsCard({ active, onSelect, onClose }: SettingsCardProps) {
  const t = useT()
  return (
    <SidebarCard
      icon={Settings}
      label={t("sidebar.settingsCard.label")}
      active={active}
      onSelect={onSelect}
      onClose={onClose}
      closeLabel={t("sidebar.settingsCard.close")}
    />
  )
}

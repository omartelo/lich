import { Settings } from "lucide-react"
import { SidebarCard } from "@/components/common/SidebarCard"

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
  return (
    <SidebarCard
      icon={Settings}
      label="Settings"
      active={active}
      onSelect={onSelect}
      onClose={onClose}
      closeLabel="Close settings"
    />
  )
}

import { FolderGit2 } from "lucide-react"
import {
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@/components/ui/dropdown-menu"
import type { ProviderState } from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"
import type { SandboxAnswer } from "@/lib/use-sandbox-choice"
import { SessionLaunchMenuItems } from "./SessionLaunchMenuItems"

/** A checkout a folder's + can open a session in, titled like its block. */
export interface LaunchCheckout {
  path: string
  label: string
}

interface FolderLaunchMenuItemsProps {
  folder: string
  /** The checkouts the folder's cards belong to; never empty, since a folder
   * exists only while a card carries its name. */
  checkouts: LaunchCheckout[]
  providers: ProviderState[]
  projectId: string
  onNewSession: (kind: ProviderKind | "shell", sandbox: SandboxAnswer, path: string) => void
}

// A folder has no directory of its own, so its + asks where the session opens,
// but only among the checkouts its cards already live in, and only when there is
// more than one: a folder of one checkout opens there, and says so.
export function FolderLaunchMenuItems({
  folder,
  checkouts,
  providers,
  projectId,
  onNewSession,
}: FolderLaunchMenuItemsProps) {
  if (checkouts.length === 1) {
    const [only] = checkouts
    return (
      <>
        <DropdownMenuGroup>
          <DropdownMenuLabel>
            New session in {folder}, <span className="whitespace-nowrap">on {only.label}</span>
          </DropdownMenuLabel>
        </DropdownMenuGroup>
        <SessionLaunchMenuItems
          providers={providers}
          terminalLabel="New Terminal"
          projectId={projectId}
          onNewSession={(kind, sandbox) => onNewSession(kind, sandbox, only.path)}
        />
      </>
    )
  }
  return (
    <DropdownMenuGroup>
      <DropdownMenuLabel>New session in {folder}</DropdownMenuLabel>
      {checkouts.map((checkout) => (
        <DropdownMenuSub key={checkout.path}>
          <DropdownMenuSubTrigger>
            <FolderGit2 />
            <span className="truncate">{checkout.label}</span>
          </DropdownMenuSubTrigger>
          <DropdownMenuSubContent className="max-w-56">
            <SessionLaunchMenuItems
              providers={providers}
              terminalLabel="New Terminal"
              projectId={projectId}
              onNewSession={(kind, sandbox) => onNewSession(kind, sandbox, checkout.path)}
            />
          </DropdownMenuSubContent>
        </DropdownMenuSub>
      ))}
    </DropdownMenuGroup>
  )
}

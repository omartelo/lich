import { Folder, GitBranch } from "lucide-react"
import { baseName, displayPath } from "@/lib/paths"

interface FooterCheckoutProps {
  path: string
  branch: string
  display?: "branch" | "path"
}

export function FooterCheckout({ path, branch, display = "branch" }: FooterCheckoutProps) {
  const showBranch = display === "branch" && branch
  const label =
    display === "path" ? displayPath(path) : branch || baseName(path) || displayPath(path)
  const Icon = showBranch ? GitBranch : Folder
  return (
    <span
      title={display === "path" ? path : label}
      className="flex min-w-0 max-w-56 items-center gap-1.5 select-text"
    >
      <Icon className="size-3.5 shrink-0" aria-hidden="true" />
      <span className="truncate">{label}</span>
    </span>
  )
}

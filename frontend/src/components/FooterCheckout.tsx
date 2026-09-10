import { Folder, GitBranch } from "lucide-react"
import { baseName, displayPath, unknownCwd } from "@/lib/paths"

/** host is set when the session's shell runs somewhere lich cannot read into:
 * the readout says so rather than naming the checkout, which is a directory the
 * user has left. The union is what confines it to the path readout: the branch
 * beside it is a fact about the checkout, true wherever the shell went, so a
 * host on a branch readout would draw "cwd unknown" under a GitBranch glyph. */
type FooterCheckoutProps = { path: string; branch: string } & (
  | { display: "path"; host?: string }
  | { display?: "branch" | "path"; host?: never }
)

export function FooterCheckout({
  path,
  branch,
  display = "branch",
  host = "",
}: FooterCheckoutProps) {
  const showBranch = display === "branch" && branch
  const label = host
    ? unknownCwd(host)
    : display === "path"
      ? displayPath(path)
      : branch || baseName(path) || displayPath(path)
  const Icon = showBranch ? GitBranch : Folder
  return (
    <span
      title={host || display !== "path" ? label : path}
      className="flex min-w-0 max-w-56 items-center gap-1.5 select-text"
    >
      <Icon className="size-3.5 shrink-0" aria-hidden="true" />
      <span className="truncate">{label}</span>
    </span>
  )
}

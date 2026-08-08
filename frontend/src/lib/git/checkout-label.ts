import { baseName } from "@/lib/paths"

// checkoutLabel titles a sidebar group with the checkout it holds: the project's
// own folder for the root group (an empty path), else the worktree's name.
//
// A worktree's name is not its last path segment. It lives at
// <worktrees-root>/<projectID>/<name> and git accepts a slash in a branch name,
// so one created as "feat/x" sits at ".../feat/x" and its final segment is "x" —
// the same header "fix/x" would draw, leaving the divider naming neither
// checkout. The project id is the one part of that root the frontend knows, and
// it is enough: everything past it is the name the worktree was made with.
//
// The name is deliberately read off the directory rather than off the branch the
// checkout currently holds. They agree only at creation; an agent that branches
// inside a worktree — the ordinary way to work in one — moves the branch and
// leaves the directory, and a header that followed it would rename a group under
// the user and repeat what every card in it already says.
//
// A checkout outside that root (a worktree added by hand elsewhere, then opened
// here) has no name to read and keeps its last segment.
export function checkoutLabel(path: string, projectPath: string, projectId: string): string {
  if (!path) {
    return baseName(projectPath)
  }
  // git reports a Windows checkout with forward slashes while lich builds one
  // with backslashes; the same path reaches this in either spelling.
  const unix = path.replace(/\\/g, "/")
  const root = `/${projectId}/`
  // The first match, not the last: a worktree may itself be named after the
  // project id, and the root is the one that comes first.
  const at = unix.indexOf(root)
  if (at === -1) {
    return baseName(path)
  }
  return unix.slice(at + root.length)
}

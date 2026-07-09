// Client-side approximation of `git check-ref-format --branch`, used for
// instant feedback while typing a worktree name. The backend re-validates with
// the real git before creating anything, so this only needs to agree with git
// on the common cases, never to be the authority.

// Control chars, space, DEL and git's forbidden set: ~ ^ : ? * [ \
const INVALID_CHARS = /[\x00-\x20\x7f~^:?*[\\]/

export function isValidBranchName(name: string): boolean {
  if (name === "" || name === "@") {
    return false
  }
  if (INVALID_CHARS.test(name)) {
    return false
  }
  if (name.includes("..") || name.includes("@{") || name.includes("//")) {
    return false
  }
  if (name.startsWith("-") || name.startsWith("/") || name.endsWith("/")) {
    return false
  }
  return name
    .split("/")
    .every(
      (part) =>
        part !== "" &&
        !part.startsWith(".") &&
        !part.endsWith(".") &&
        !part.endsWith(".lock"),
    )
}

// displayPath collapses a user's home directory prefix to "~", the way most
// shells and terminals render paths: "/home/me/try/skipo" -> "~/try/skipo".
// Paths outside a home directory are returned unchanged.
const HOME_ROOTS = /^(?:\/home\/[^/]+|\/Users\/[^/]+|[A-Za-z]:\\Users\\[^\\]+)/

export function displayPath(path: string): string {
  return path.replace(HOME_ROOTS, "~")
}

// baseName returns the final segment of a path — the folder name — tolerating
// either separator and a trailing slash. Empty input (and a bare "/") returns "".
export function baseName(path: string): string {
  const segments = path.split(/[/\\]+/).filter(Boolean)
  return segments[segments.length - 1] ?? ""
}

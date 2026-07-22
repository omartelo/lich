// Language badges for diff file headers: a short abbreviation plus Tailwind
// classes for the colored square, keyed by file extension.

export interface LangBadge {
  abbr: string
  className: string
}

const BADGES: Record<string, LangBadge> = {
  ts: {abbr: "TS", className: "bg-blue-500/15 text-blue-600 dark:text-blue-400"},
  tsx: {abbr: "TS", className: "bg-blue-500/15 text-blue-600 dark:text-blue-400"},
  js: {abbr: "JS", className: "bg-yellow-500/15 text-yellow-600 dark:text-yellow-400"},
  jsx: {abbr: "JS", className: "bg-yellow-500/15 text-yellow-600 dark:text-yellow-400"},
  go: {abbr: "GO", className: "bg-cyan-500/15 text-cyan-600 dark:text-cyan-400"},
  css: {abbr: "CSS", className: "bg-sky-500/15 text-sky-600 dark:text-sky-400"},
  html: {abbr: "HTM", className: "bg-orange-500/15 text-orange-600 dark:text-orange-400"},
  json: {abbr: "{}", className: "bg-amber-500/15 text-amber-600 dark:text-amber-400"},
  md: {abbr: "MD", className: "bg-violet-500/15 text-violet-600 dark:text-violet-400"},
  yaml: {abbr: "YML", className: "bg-rose-500/15 text-rose-600 dark:text-rose-400"},
  yml: {abbr: "YML", className: "bg-rose-500/15 text-rose-600 dark:text-rose-400"},
  sh: {abbr: "SH", className: "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400"},
  py: {abbr: "PY", className: "bg-teal-500/15 text-teal-600 dark:text-teal-400"},
  rs: {abbr: "RS", className: "bg-orange-500/15 text-orange-600 dark:text-orange-400"},
  sql: {abbr: "SQL", className: "bg-indigo-500/15 text-indigo-600 dark:text-indigo-400"},
  toml: {abbr: "TML", className: "bg-stone-500/15 text-stone-600 dark:text-stone-400"},
}

const FALLBACK_CLASS = "bg-muted text-muted-foreground"

// extname returns a path's lowercased extension without the dot, or "" for
// extensionless files and dotfiles (a leading dot is not an extension). The one
// source of extension parsing, shared by the badge and the file-tree icon.
export function extname(path: string): string {
  const base = path.slice(path.lastIndexOf("/") + 1)
  const dot = base.lastIndexOf(".")
  return dot > 0 ? base.slice(dot + 1).toLowerCase() : ""
}

// languageAbbr picks the badge for a path by extension; unknown extensions get
// their first letters uppercased, extensionless files a neutral dot.
export function languageAbbr(path: string): LangBadge {
  const ext = extname(path)
  if (ext in BADGES) {
    return BADGES[ext]
  }
  return {
    abbr: ext ? ext.slice(0, 3).toUpperCase() : "•",
    className: FALLBACK_CLASS,
  }
}

// splitPath separates a repo-relative path into directory and basename for the
// two-tone file header ("App.tsx" bold, "src/components" muted).
export function splitPath(path: string): { dir: string; base: string } {
  const slash = path.lastIndexOf("/")
  return slash < 0
    ? {dir: "", base: path}
    : {dir: path.slice(0, slash), base: path.slice(slash + 1)}
}

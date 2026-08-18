// formatModel turns a raw Claude model id into the short label the tooltip
// shows: drop the "claude-" prefix and any trailing date snapshot, then render
// the version dash-segments as a dotted number after the family word —
// "claude-opus-4-8" → "opus 4.8", "claude-haiku-4-5-20251001" → "haiku 4.5",
// "claude-fable-5" → "fable 5". An id that doesn't fit the shape (no family +
// version split) is shown stripped but otherwise untouched.
export function formatModel(id: string): string {
  if (!id.startsWith("claude-")) {
    return id
  }
  const stripped = id.replace(/^claude-/, "").replace(/-\d{8}$/, "")
  const [family, ...version] = stripped.split("-")
  return version.length > 0 ? `${family} ${version.join(".")}` : stripped
}

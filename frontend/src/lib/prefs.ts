// UI preferences live in the page's localStorage (see the root CLAUDE.md:
// the workspace is SQLite, the prefs are not). What each stored string means
// is the parsing half, kept pure so the suite owns it; reading and writing the
// key itself is the boundary half, and stays a two-line wrapper — the same
// split parsePullsSort/readPullsSort already uses.
//
// Every reader answers with the fallback for a key that is absent, or that
// holds something this build no longer understands. A pref is a convenience:
// it must never be able to break a launch.

export function readPref(key: string): string | null {
  return localStorage.getItem(key)
}

export function writePref(key: string, value: string | number | boolean): void {
  localStorage.setItem(key, String(value))
}

export function removePref(key: string): void {
  localStorage.removeItem(key)
}

/** One of a known set — a theme, a sort. Anything else is a value from another
 * build (or a hand-edited store) and reads as the fallback. */
export function parseEnumPref<T extends string>(
  raw: string | null,
  allowed: readonly T[],
  fallback: T,
): T {
  return allowed.find((value) => value === raw) ?? fallback
}

/** A numeric pref, put through the setting's own clamp so a value from a build
 * with different bounds lands inside this one's. A non-positive number is
 * treated as absent rather than clamped: both numeric prefs are magnitudes, so
 * a 0 is garbage, and clamping it to the minimum would hide that. */
export function parseNumberPref(
  raw: string | null,
  fallback: number,
  clamp: (value: number) => number,
): number {
  const value = Number(raw)
  return raw !== null && Number.isFinite(value) && value > 0 ? clamp(value) : fallback
}

/** A flag. Only what writePref emits for a boolean counts; anything else is
 * the fallback, so a half-written or foreign value cannot pin a toggle. */
export function parseBoolPref(raw: string | null, fallback: boolean): boolean {
  if (raw === "true") {
    return true
  }
  if (raw === "false") {
    return false
  }
  return fallback
}

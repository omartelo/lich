// The last answer of every backend read that asked to be remembered, kept for
// the next mount. The screens of this app are routes, so walking away from one
// destroys it: the pull request screen alone is five gh round-trips, and coming
// back to a review in progress used to cost all five, with a skeleton in their
// place while they ran.
//
// A cache, not a store: nothing here is a source of truth. Whoever reads an
// entry refetches behind it and replaces it with the answer
// (use-remote-resource) — this only decides what is on screen while that runs.
//
// Module-level and never persisted. It survives navigation, which is the whole
// problem; it does not survive a reload, where a fresh page and fresh answers
// are what was asked for.

// How many answers are held at once. Diffs are the large ones and there is no
// byte budget here, only a count — a review that walks through more pull
// requests than this loses the oldest, and paints a skeleton on returning to
// it, which is exactly where the screen was before this file existed.
const MAX_ENTRIES = 32

const entries = new Map<string, unknown>()

/** The last answer filed under this key, or undefined for one never answered. */
export function readRemoteCache<T>(key: string): T | undefined {
  return entries.get(key) as T | undefined
}

/** File an answer, evicting the least recently filed once the cache is full. */
export function writeRemoteCache(key: string, value: unknown): void {
  // A Map iterates in insertion order, so deleting before setting is what makes
  // that order a recency order — and the first key the one eviction should take.
  entries.delete(key)
  entries.set(key, value)
  if (entries.size > MAX_ENTRIES) {
    const oldest = entries.keys().next()
    if (!oldest.done) {
      entries.delete(oldest.value)
    }
  }
}

/** Drop everything. For the suite, which must not read another test's answers. */
export function clearRemoteCache(): void {
  entries.clear()
}

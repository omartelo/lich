// Client-side approximation of `git check-ref-format --branch`, used for
// instant feedback while typing a worktree name. The backend re-validates with
// the real git before creating anything, so this only needs to agree with git
// on the common cases, never to be the authority.

// Control chars, space, DEL and git's forbidden set: ~ ^ : ? * [ \
// biome-ignore lint/suspicious/noControlCharactersInRegex: rejecting control characters is the point — git forbids them in a ref name.
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
        part !== "" && !part.startsWith(".") && !part.endsWith(".") && !part.endsWith(".lock"),
    )
}

// A name typed as words is grown one word at a time: to at least a minimum, so
// a run of very short words still reads as a phrase, and stopped before a
// maximum, so a pasted sentence does not become the branch. Both bounds are
// counted in words and in characters — either one alone lets the other run away.
const MIN_WORDS = 2
const MAX_WORDS = 5
const MIN_CHARS = 10
const MAX_CHARS = 40

/**
 * The branch a typed worktree name creates: what was typed when git already
 * accepts it, a kebab-case slug of the first few words when it does not, and
 * "" when nothing usable is left — which is the same blank the caller
 * auto-generates a name for.
 *
 * The point is the name a branch carries into a pull request. A blank field
 * takes the random adjective-noun, and renaming that branch after the PR is
 * open closes the PR, so the only cheap moment to say what the work is called
 * is before the worktree exists. Typing the task in plain words is that moment.
 */
export function toBranchName(input: string): string {
  const trimmed = input.trim()
  if (trimmed === "" || isValidBranchName(trimmed)) {
    return trimmed
  }
  // Letters and numbers in any script, not just ASCII: git takes a non-ASCII
  // branch name, and stripping the accents off a phrase would spell it wrong.
  const words = trimmed
    .toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, " ")
    .trim()
    .split(/\s+/)
    .filter(Boolean)
  const picked: string[] = []
  let length = 0
  for (const word of words) {
    const next = length === 0 ? word.length : length + 1 + word.length
    const haveMinimum = picked.length >= MIN_WORDS && length >= MIN_CHARS
    if (picked.length >= MAX_WORDS || (next > MAX_CHARS && haveMinimum)) {
      break
    }
    picked.push(word)
    length = next
  }
  // A single word longer than the maximum is picked before the bound applies —
  // it is that or nothing — so the clamp is enforced here, on the joined name.
  return picked.join("-").slice(0, MAX_CHARS).replace(/-+$/, "")
}

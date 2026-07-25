/**
 * Pure helpers for the "copied to clipboard" toast shown when the user selects
 * text in the terminal. Framework-free so it can be unit-tested without
 * rendering the terminal or the toast host.
 */

export const COPY_TOAST_DURATION_MS = 1500

/** Count user-perceived characters (code points, not UTF-16 code units). */
export function countChars(text: string): number {
  return [...text].length
}

export function copyToastMessage(text: string): string {
  const count = countChars(text)
  return `copied ${count} ${count === 1 ? "char" : "chars"} to clipboard`
}

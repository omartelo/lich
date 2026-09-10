// What the update toast shows for one progress step, kept pure (no toast, no
// socket) so the arithmetic and the wording are testable. The component
// renders these.

import type { AppUpdateProgress } from "@/lib/api-types"

/** internal/appupdate.ProgressEventName. */
export const UPDATE_PROGRESS_EVENT = "appupdate-progress"

const BYTES_PER_MB = 1_000_000

export function isUpdateProgress(data: unknown): data is AppUpdateProgress {
  if (typeof data !== "object" || data === null) return false
  const p = data as Record<string, unknown>
  return (
    (p.phase === "download" || p.phase === "install" || p.phase === "installer") &&
    typeof p.received === "number" &&
    typeof p.total === "number"
  )
}

/** Megabytes with one decimal: the assets run 10–120 MB, so no smaller unit. */
export function formatMegabytes(bytes: number): string {
  return `${(bytes / BYTES_PER_MB).toFixed(1)} MB`
}

/** DownloadReading is the bar and its caption: percent is null while the total
 * is unknown, which draws the bar indeterminate. */
export interface DownloadReading {
  percent: number | null
  bytes: string
}

export function downloadReading(p: AppUpdateProgress): DownloadReading {
  if (p.total <= 0) {
    return { percent: null, bytes: formatMegabytes(p.received) }
  }
  const percent = Math.min(100, Math.floor((p.received * 100) / p.total))
  return { percent, bytes: `${formatMegabytes(p.received)} of ${formatMegabytes(p.total)}` }
}

/** failureText names the phase the update died in — there are two now — and,
 * for a download, how far it got. */
export function failureText(last: AppUpdateProgress | null, error: string): string {
  if (last === null || last.phase !== "download") {
    return last === null ? `Update failed: ${error}` : `Install failed: ${error}`
  }
  const { percent } = downloadReading(last)
  return percent === null ? `Download failed: ${error}` : `Download failed at ${percent}%: ${error}`
}

import { LoaderCircle } from "lucide-react"
import type { AppUpdateProgress } from "@/lib/api-types"
import { downloadReading } from "@/lib/update/update-progress"

// UpdateProgressToast is the body of the update toast while Apply runs: a bar
// with percent and bytes during the download (indeterminate without a total),
// then a spinner naming the phase that has no percentage. Rendered through
// toast.custom on the popover surface, like the install prompt.
export function UpdateProgressToast({
  version,
  progress,
}: {
  version: string
  progress: AppUpdateProgress | null
}) {
  const downloading = progress === null || progress.phase === "download"
  return (
    <div className="flex w-full flex-col gap-2.5 rounded-md border bg-popover p-4 text-sm text-popover-foreground shadow-lg">
      <div className="flex items-center gap-2">
        {!downloading && <LoaderCircle className="size-4 shrink-0 animate-spin" />}
        <span className="min-w-0 flex-1 truncate font-medium">
          {downloading ? `Downloading lich ${version}` : `Installing lich ${version}…`}
        </span>
      </div>
      {downloading && <DownloadBar progress={progress} />}
      {progress?.phase === "installer" && (
        <span className="text-xs text-muted-foreground">lich closes and reopens on its own.</span>
      )}
    </div>
  )
}

function DownloadBar({ progress }: { progress: AppUpdateProgress | null }) {
  const reading = progress ? downloadReading(progress) : null
  const percent = reading?.percent ?? null
  return (
    <>
      <span className="h-1.5 overflow-hidden rounded-full bg-muted">
        {percent === null ? (
          <span className="block h-full w-1/3 animate-update-indeterminate rounded-full bg-foreground motion-reduce:animate-none" />
        ) : (
          <span
            className="block h-full rounded-full bg-foreground"
            style={{ width: `${percent}%` }}
          />
        )}
      </span>
      <span className="flex justify-between text-xs text-muted-foreground tabular-nums">
        <span>{percent === null ? "" : `${percent}%`}</span>
        <span>{reading?.bytes ?? ""}</span>
      </span>
    </>
  )
}

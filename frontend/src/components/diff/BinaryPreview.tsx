import { useState } from "react"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import type { DiffFile } from "@/lib/git/diff"
import { type BlobSides, formatBlobSize, previewKind } from "@/lib/git/preview"
import { useBlob } from "@/lib/git/use-blob"
import { useT } from "@/lib/i18n/i18n"
import { cn } from "@/lib/utils"

type SideTone = "del" | "add"
type SideLabelKey = `diff.binaryPreview.${"before" | "after" | "added" | "deleted"}`

interface SideSpec {
  label: SideLabelKey
  tone: SideTone
  rel: string
  ref: string
}

// sidesOf names the columns a change has: an added file has no old side and a
// deleted one no new side, so neither draws an empty column.
function sidesOf(file: DiffFile, sides: BlobSides): SideSpec[] {
  const before = {
    label: "diff.binaryPreview.before",
    tone: "del",
    rel: file.oldPath,
    ref: sides.before,
  } as const
  const after = {
    label: "diff.binaryPreview.after",
    tone: "add",
    rel: file.newPath,
    ref: sides.after,
  } as const
  if (file.status === "added") {
    return [{ ...after, label: "diff.binaryPreview.added" }]
  }
  if (file.status === "deleted") {
    return [{ ...before, label: "diff.binaryPreview.deleted" }]
  }
  return [before, after]
}

interface BinaryDiffBodyProps {
  file: DiffFile
  /** Absent where the diff has no revisions lich can read (a pull request). */
  sides?: BlobSides
}

/** BinaryDiffBody is a binary file's diff: the image or PDF on each side, or
 * the line saying why there is nothing to draw. */
export function BinaryDiffBody({ file, sides }: BinaryDiffBodyProps) {
  const t = useT()
  const kind = previewKind(file.newPath)
  if (!sides || !kind) {
    return (
      <BinaryHint>
        {sides ? t("diff.binaryPreview.binaryNoPreview") : t("diff.binaryPreview.binary")}
      </BinaryHint>
    )
  }
  const specs = sidesOf(file, sides)
  if (kind === "pdf") {
    return <PdfDiff path={sides.path} specs={specs} version={file.blobIds} />
  }
  return (
    <div className={cn("grid gap-2.5 px-3 pt-2.5 pb-3.5", specs.length === 2 && "grid-cols-2")}>
      {specs.map((spec) => (
        <div key={spec.label} className="flex min-w-0 flex-col gap-1">
          <SideLabel label={spec.label} tone={spec.tone} />
          <ImageView path={sides.path} rel={spec.rel} gitRef={spec.ref} version={file.blobIds} />
        </div>
      ))}
    </div>
  )
}

function BinaryHint({ children }: { children: string }) {
  return <p className="px-9 py-2 text-xs text-muted-foreground">{children}</p>
}

function SideLabel({ label, tone }: { label: SideLabelKey; tone: SideTone }) {
  const t = useT()
  return (
    <span className="flex items-center gap-1.5 text-2xs uppercase tracking-wide text-muted-foreground">
      <span
        className={cn("size-1.5 rounded-full", tone === "add" ? "bg-tone-pass" : "bg-destructive")}
      />
      {t(label)}
    </span>
  )
}

interface BlobViewProps {
  path: string
  rel: string
  /** A revision in project.Blob's terms; named apart from React's own `ref`. */
  gitRef: string
  version: string
}

/** ImageView draws one image on the checkerboard with its dimensions and size
 * under it, or the backend's reason in its place. */
export function ImageView({ path, rel, gitRef, version }: BlobViewProps) {
  const t = useT()
  const read = useBlob(path, rel, gitRef, version)
  const [size, setSize] = useState<{ width: number; height: number } | null>(null)
  if (read === null) {
    return <p className="text-xs text-muted-foreground">{t("diff.binaryPreview.loading")}</p>
  }
  if (read.state !== "ok") {
    return (
      <p className="text-xs text-muted-foreground">
        {read.state === "missing" ? t("diff.binaryPreview.missing") : read.message}
      </p>
    )
  }
  return (
    <>
      <div className="preview-checker flex items-center justify-center overflow-hidden rounded-sm">
        <img
          src={read.url}
          alt={rel}
          className="block max-h-[32rem] max-w-full object-contain"
          onLoad={(event) =>
            setSize({
              width: event.currentTarget.naturalWidth,
              height: event.currentTarget.naturalHeight,
            })
          }
          onError={() => setSize(null)}
        />
      </div>
      <span className="font-mono text-[0.625rem] text-muted-foreground tabular-nums">
        {size && <span className="text-foreground">{`${size.width} × ${size.height}`} · </span>}
        {formatBlobSize(read.bytes)}
      </span>
    </>
  )
}

/** PdfView is Chromium's own PDF viewer on one side, which brings its zoom,
 * search and pages. className sizes it: the diff gives it a fixed height, the
 * Files tab the whole panel. */
export function PdfView({
  path,
  rel,
  gitRef,
  version,
  className,
}: BlobViewProps & { className?: string }) {
  const t = useT()
  const read = useBlob(path, rel, gitRef, version)
  if (read === null) {
    return <p className="text-xs text-muted-foreground">{t("diff.binaryPreview.loading")}</p>
  }
  if (read.state !== "ok") {
    return (
      <p className="text-xs text-muted-foreground">
        {read.state === "missing" ? t("diff.binaryPreview.missing") : read.message}
      </p>
    )
  }
  return <iframe src={read.url} title={rel} className={cn("w-full border-0", className)} />
}

// PdfDiff shows one side at a time: two viewers side by side are unreadable at
// the dock's width, so a changed PDF swaps sides under a toggle instead.
function PdfDiff({ path, specs, version }: { path: string; specs: SideSpec[]; version: string }) {
  const t = useT()
  const [shown, setShown] = useState(specs.length - 1)
  const spec = specs[shown] ?? specs[0]
  return (
    <div className="flex flex-col gap-2 px-3 pt-2.5 pb-3.5">
      {specs.length === 2 ? (
        <ToggleGroup
          value={[String(shown)]}
          onValueChange={(next) => next[0] && setShown(Number(next[0]))}
          spacing={1}
          aria-label={t("diff.binaryPreview.pdfSide")}
          className="self-start border border-border p-[0.1875rem]"
        >
          {specs.map((side, index) => (
            <ToggleGroupItem
              key={side.label}
              value={String(index)}
              size="sm"
              className="h-6 px-2.5 text-xs"
            >
              {t(side.label)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      ) : (
        <SideLabel label={spec.label} tone={spec.tone} />
      )}
      <PdfView
        path={path}
        rel={spec.rel}
        gitRef={spec.ref}
        version={version}
        className="h-[21rem]"
      />
    </div>
  )
}

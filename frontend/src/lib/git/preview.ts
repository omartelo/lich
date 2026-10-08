import { type Endpoint, endpoint } from "@/lib/rpc"

export type PreviewKind = "image" | "pdf"

// Mirrors internal/project.previewTypes, which refuses everything else.
const IMAGE_EXTENSIONS = new Set(["avif", "bmp", "gif", "ico", "jpeg", "jpg", "png", "webp"])

/** Which preview a repo-relative path gets, by extension; null for none. */
export function previewKind(rel: string): PreviewKind | null {
  const name = rel.slice(rel.lastIndexOf("/") + 1)
  const dot = name.lastIndexOf(".")
  if (dot <= 0) {
    return null
  }
  const extension = name.slice(dot + 1).toLowerCase()
  if (extension === "pdf") {
    return "pdf"
  }
  return IMAGE_EXTENSIONS.has(extension) ? "image" : null
}

/** The revisions a preview reads its two sides at, in project.Blob's terms:
 * "" is the working tree, "HEAD" the commit under it, an oid a snapshot tree. */
export interface BlobSides {
  /** The checkout the paths are relative to. */
  path: string
  before: string
  after: string
}

export function blobUrl({ base, token }: Endpoint, path: string, rel: string, ref: string): string {
  return `${base}/blob?${new URLSearchParams({ token, path, rel, ref })}`
}

export type BlobRead =
  | { state: "ok"; url: string; bytes: number }
  /** The side has no file: the old side of an added one, the new of a deleted. */
  | { state: "missing" }
  /** The backend's reason, worded for the page ("this file type can't be previewed"). */
  | { state: "refused"; message: string }

/** Fetches one side into an object URL, which the caller revokes. Fetched
 * rather than pointed at by <img src> so a refusal has its status and words. */
export async function readBlob(path: string, rel: string, ref: string): Promise<BlobRead> {
  const response = await fetch(blobUrl(endpoint(), path, rel, ref))
  if (response.status === 404) {
    return { state: "missing" }
  }
  if (!response.ok) {
    return { state: "refused", message: (await response.text()).trim() }
  }
  const blob = await response.blob()
  return { state: "ok", url: URL.createObjectURL(blob), bytes: blob.size }
}

const BYTES_PER_KB = 1024
const BYTES_PER_MB = 1024 * 1024

/** A preview's size caption: KB below a megabyte, one decimal either way. */
export function formatBlobSize(bytes: number): string {
  if (bytes < BYTES_PER_MB) {
    return `${(bytes / BYTES_PER_KB).toFixed(1)} KB`
  }
  return `${(bytes / BYTES_PER_MB).toFixed(1)} MB`
}

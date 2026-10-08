import { useEffect, useState } from "react"
import { errorText } from "@/lib/utils"
import { type BlobRead, readBlob } from "./preview"

/** One preview side, null while it loads. version is what changes when the
 * bytes behind the same path and ref do (a diff's blob ids): the working tree
 * answers differently under the same URL as the agent writes. */
export function useBlob(path: string, rel: string, ref: string, version: string): BlobRead | null {
  const [read, setRead] = useState<BlobRead | null>(null)
  useEffect(() => {
    let live = true
    let url = ""
    setRead(null)
    readBlob(path, rel, ref).then(
      (next) => {
        if (next.state === "ok") {
          url = next.url
        }
        if (live) {
          setRead(next)
        } else if (url) {
          URL.revokeObjectURL(url)
        }
      },
      (err: unknown) => live && setRead({ state: "refused", message: errorText(err) }),
    )
    return () => {
      live = false
      if (url) {
        URL.revokeObjectURL(url)
      }
    }
  }, [path, rel, ref, version])
  return read
}

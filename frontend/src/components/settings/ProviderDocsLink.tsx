import { ExternalLink } from "lucide-react"
import type { ProviderState } from "@/lib/providers-store"
import { System } from "@/lib/rpc"

// What a provider lich could not find owes the user: the page that documents
// installing it. Opened in the real browser (System.OpenExternal), like every
// other outbound link in the app — the Chromium shell has no second window to
// hand it to.
export function ProviderDocsLink({ provider }: { provider: ProviderState }) {
  if (provider.docs === "") {
    return null
  }
  return (
    <button
      type="button"
      className="inline-flex items-center gap-1 underline-offset-2 hover:text-foreground hover:underline"
      onClick={() => void System.OpenExternal(provider.docs)}
    >
      How to install
      <ExternalLink className="size-3" aria-label={`Install ${provider.name}`} />
    </button>
  )
}

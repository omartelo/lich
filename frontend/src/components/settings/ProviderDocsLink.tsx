import { ExternalLink } from "lucide-react"
import type { ProviderState } from "@/lib/providers-store"
import { useT } from "@/lib/i18n/i18n"
import { System } from "@/lib/rpc"

// What a provider lich could not find owes the user: the page that documents
// installing it. Opened in the real browser (System.OpenExternal), like every
// other outbound link in the app — the Chromium shell has no second window to
// hand it to.
//
// The same page is worth offering for a provider that *is* installed, where the
// question is how to drive it rather than how to get it, so the label is the
// caller's: it is the only thing that differs between the two.
export function ProviderDocsLink({ provider, label }: { provider: ProviderState; label?: string }) {
  const t = useT()
  if (provider.docs === "") {
    return null
  }
  return (
    <button
      type="button"
      className="inline-flex items-center gap-1 underline-offset-2 hover:text-foreground hover:underline"
      onClick={() => void System.OpenExternal(provider.docs)}
    >
      {label ?? t("settings.providerDocsLink.howToInstall")}
      <ExternalLink className="size-3" aria-label={provider.name} />
    </button>
  )
}

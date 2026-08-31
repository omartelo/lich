import { RefreshCw } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { refreshProviders } from "@/lib/providers-store"
import { errorText } from "@/lib/utils"

// The re-probe both provider surfaces offer, so an agent installed with lich
// already open shows up without a relaunch. It re-runs the PATH scan only — not
// the login shell that produced that PATH, which would hang the button as
// readily as it hangs a boot (docs/ceilings.md).
export function ProviderRefreshButton({ size = "sm" }: { size?: "sm" | "default" }) {
  const [checking, setChecking] = useState(false)

  // The detect RPC can fail, and the store rethrows when it does. Unhandled, the
  // spinner just stopped: a button that says "Check again" and answers nothing
  // reads as "nothing was installed", which is the wrong thing to have learned.
  const check = async () => {
    setChecking(true)
    try {
      await refreshProviders()
    } catch (error) {
      toast.error(`Couldn't check for providers: ${errorText(error)}`)
    } finally {
      setChecking(false)
    }
  }

  return (
    <Button variant="ghost" size={size} disabled={checking} onClick={() => void check()}>
      <RefreshCw data-icon="inline-start" className={checking ? "animate-spin" : undefined} />
      {checking ? "Checking…" : "Check again"}
    </Button>
  )
}

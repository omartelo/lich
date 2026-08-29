import { RefreshCw } from "lucide-react"
import { useState } from "react"
import { Button } from "@/components/ui/button"
import { refreshProviders } from "@/lib/providers-store"

// The re-probe both provider surfaces offer, so an agent installed with lich
// already open shows up without a relaunch. It re-runs the PATH scan only — not
// the login shell that produced that PATH, which would hang the button as
// readily as it hangs a boot (docs/ceilings.md).
export function ProviderRefreshButton({ size = "sm" }: { size?: "sm" | "default" }) {
  const [checking, setChecking] = useState(false)

  const check = () => {
    setChecking(true)
    void refreshProviders().finally(() => setChecking(false))
  }

  return (
    <Button variant="ghost" size={size} disabled={checking} onClick={check}>
      <RefreshCw data-icon="inline-start" className={checking ? "animate-spin" : undefined} />
      {checking ? "Checking…" : "Check again"}
    </Button>
  )
}

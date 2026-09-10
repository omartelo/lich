import { RefreshCw } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { refreshPath } from "@/lib/path-refresh"
import { errorText } from "@/lib/utils"

interface CheckAgainButtonProps {
  /** The surface's own check, run once the $PATH has been re-read. Omitted
   * where the surface draws from useBinaryCheck, which wakes on its own. */
  onCheck?: () => Promise<void>
  size?: "sm" | "default"
}

// The re-check every "installed, but lich cannot see it" surface offers: it
// re-reads the login shell's $PATH in the backend and replaces the pin every
// binary lookup resolves through (lib/path-refresh), so an agent, git or gh
// installed with lich already open is found without a relaunch.
export function CheckAgainButton({ onCheck, size = "sm" }: CheckAgainButtonProps) {
  const [checking, setChecking] = useState(false)

  // Both halves can fail, and unhandled the spinner just stops: a button that
  // says "Check again" and answers nothing reads as "nothing was installed",
  // which is the wrong thing to have learned.
  const check = async () => {
    setChecking(true)
    try {
      await refreshPath()
      await onCheck?.()
    } catch (error) {
      toast.error(errorText(error))
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

import { useCallback, useSyncExternalStore } from "react"
import type { QuotaPlan } from "@/lib/api-types"
import { planQuotaStore } from "./plan-quota-store"

// usePlanQuota reads the plan usage of every provider that meters a
// subscription, for the account sessionId spends — an empty sessionId reads the
// machine's own login, which is the settings screen's question. Empty until the
// first reading lands, and empty forever for a machine signed in to none of
// them.
export function usePlanQuota(sessionId: string): QuotaPlan[] {
  const subscribe = useCallback(
    (listener: () => void) => planQuotaStore.subscribe(sessionId, listener),
    [sessionId],
  )
  const snapshot = useCallback(() => planQuotaStore.getSnapshot(sessionId), [sessionId])
  return useSyncExternalStore(subscribe, snapshot)
}

// usePlanQuotaFor is one provider's reading, by its provider id — null for a
// provider that meters nothing (opencode, oh-my-pi and Crush run on the user's
// own API keys) and while the first reading is in flight.
export function usePlanQuotaFor(provider: string | undefined, sessionId = ""): QuotaPlan | null {
  const plans = usePlanQuota(sessionId)
  if (!provider) {
    return null
  }
  return plans.find((plan) => plan.provider === provider) ?? null
}

import { useSyncExternalStore } from "react"
import type { QuotaPlan } from "@/lib/api-types"
import { planQuotaStore } from "./plan-quota-store"

// usePlanQuota reads the plan usage of every provider that meters a
// subscription. Empty until the first reading lands, and empty forever for a
// machine signed in to none of them.
export function usePlanQuota(): QuotaPlan[] {
  return useSyncExternalStore(planQuotaStore.subscribe, planQuotaStore.getSnapshot)
}

// usePlanQuotaFor is one provider's reading, by its provider id — null for a
// provider that meters nothing (opencode, oh-my-pi and Crush run on the user's
// own API keys) and while the first reading is in flight.
export function usePlanQuotaFor(provider: string | undefined): QuotaPlan | null {
  const plans = usePlanQuota()
  if (!provider) {
    return null
  }
  return plans.find((plan) => plan.provider === provider) ?? null
}

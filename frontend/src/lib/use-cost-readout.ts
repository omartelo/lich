import { useSyncExternalStore } from "react"
import { costReadoutStore } from "./cost-readout-store"

// useCostReadout reports whether the per-session cost readout is on. False
// until the stored setting has been read, so the footer never flashes a number
// at someone who turned it off.
export function useCostReadout(): boolean {
  return useSyncExternalStore(costReadoutStore.subscribe, costReadoutStore.get)
}

// A layout edit must wait for the old cost choice before saving its migration.
export function useCostReadoutReady(): boolean {
  return useSyncExternalStore(costReadoutStore.subscribeReady, costReadoutStore.isReady)
}

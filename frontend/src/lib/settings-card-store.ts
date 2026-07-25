import { createCardStore } from "./card-store"

// Tracks which projects have their Settings card parked in the sidebar, keyed by
// project id. See card-store for the shape and the lifetime.
const store = createCardStore()

export const openSettings = store.open
export const closeSettings = store.close
export const isSettingsOpen = store.isOpen
export const subscribeSettingsCard = store.subscribe

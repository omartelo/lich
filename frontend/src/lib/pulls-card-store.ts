import { createCardStore } from "./card-store"

// Tracks which worktree checkouts have their Pull request card parked in the
// sidebar, keyed by the checkout path — each worktree parks its own. See
// card-store for the shape and the lifetime.
const store = createCardStore()

export const openPulls = store.open
export const closePulls = store.close
export const isPullsOpen = store.isOpen
export const subscribePullsCard = store.subscribe

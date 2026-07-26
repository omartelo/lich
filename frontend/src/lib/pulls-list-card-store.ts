import { createCardStore } from "./card-store"

// Tracks whether a project has the repository-wide "Pull requests" card parked
// in the sidebar, keyed by project id. Distinct from pulls-card-store, which is
// keyed by checkout path: that card belongs to one worktree and opens that
// worktree's own pull request, while this one belongs to the project and opens
// the list. See card-store for the shape and the lifetime.
const store = createCardStore()

export const openPullsList = store.open
export const closePullsList = store.close
export const isPullsListOpen = store.isOpen
export const subscribePullsListCard = store.subscribe

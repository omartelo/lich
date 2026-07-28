// The per-project settings keys, mirroring internal/store/settings.go. Hand-
// owned like api-types.ts: the Go side resolves these same strings on its own
// (it reads them for the gh token and the worktree spawn, not only when the UI
// asks), so a rename there is a rename here in the same change.
//
// They live in lib rather than beside the settings panes that write them
// because the contract is the backend's, not the pane's.

/** Which authenticated GitHub account lich's gh calls run as for a project. */
export const GH_ACCOUNT_KEY = "vcs.account"

/** The script a fresh worktree runs before its provider starts. */
export const WORKTREE_SETUP_KEY = "worktree.setup"

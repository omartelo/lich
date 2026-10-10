// The text lich types into an agent's terminal. Rendered in the prompt
// language (prompt-language-store.ts), never the interface language: use
// `prompt()` from lib/i18n/prompt.ts, not t().
export const prompts = {
  pullRequest: {
    conflicts: "Pull request #{number} ({branch}) has merge conflicts with {base}. Resolve them.",
    checksUnnamed: {
      one: "CI is failing on pull request #{number} ({branch}). {count} check is red; find out which and fix them.",
      other:
        "CI is failing on pull request #{number} ({branch}). {count} checks are red; find out which and fix them.",
    },
    checksListed:
      "CI is failing on pull request #{number} ({branch}). Fix these checks:\n\n{checks}",
    checksOmitted: {
      one: "\n\n…and {count} more failing check not listed.",
      other: "\n\n…and {count} more failing checks not listed.",
    },
    create:
      "Branch {branch} has no pull request yet. Open one with `gh pr create`: push the branch first if it is not on the remote, write the title and body from the commits and the diff against the base branch, and follow the repository's pull request template if it has one.",
  },
  delegate: {
    session: 'Delegate to the "{target}" session: ',
    newWorktree: "Delegate to a new worktree session: ",
    newWorktreeWithCommand:
      "Delegate to a new worktree session (run `lich open --worktree <branch> --prompt <task>`): ",
  },
  reviewComments: {
    header: "Review comments:",
  },
  issue: {
    head: "GitHub issue #{number} — {title}\n{url}",
  },
} as const

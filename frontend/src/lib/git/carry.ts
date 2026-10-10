import { toast } from "sonner"
import { ProjectService } from "@/lib/rpc"
import type { Worktree } from "@/lib/api-types"
import { errorText } from "@/lib/utils"
import { t } from "@/lib/i18n/i18n"

// carryInto copies a checkout's uncommitted work (a forked session's, or the
// project's own picked from the + dialog) into the worktree that was just
// created for it, and says out loud when it cannot. It never throws:
// the worktree exists either way, and the session opening on it is worth more
// than the copy — the work it was carrying is still sitting in the checkout it
// came from.
//
// from is "" for every worktree that was not based on a working-tree row, which
// is most of them.
export async function carryInto(from: string, wt: Worktree): Promise<void> {
  if (!from) {
    return
  }
  // A branch that already existed is checked out as it stands and the base is
  // ignored (project.CreateWorktree), so the commit this patch was read against
  // is not the one the new checkout sits on. git would refuse the patch whole;
  // saying which of the two things the user asked for did not happen is the
  // part git cannot do.
  if (wt.reused) {
    toast.warning(t("git.carry.reused", { name: wt.name }))
    return
  }
  try {
    await ProjectService.CarryUncommitted(from, wt.path)
  } catch (err: unknown) {
    toast.error(t("git.carry.failed", { error: errorText(err) }))
  }
}

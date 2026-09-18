import { toast } from "sonner"
import { ProjectService } from "@/lib/rpc"
import type { Worktree } from "@/lib/api-types"
import { errorText } from "@/lib/utils"

// carryInto copies a forked session's uncommitted work into the worktree that
// was just created for it, and says out loud when it cannot. It never throws:
// the worktree exists either way, and the session opening on it is worth more
// than the copy — the work it was carrying is still sitting in the card it came
// from.
//
// from is "" for every worktree that is not a fork of a working tree, which is
// most of them.
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
    toast.warning(
      `${wt.name} already existed, so it was checked out as it stands — the uncommitted work was not carried over.`,
    )
    return
  }
  try {
    await ProjectService.CarryUncommitted(from, wt.path)
  } catch (err: unknown) {
    toast.error(`Couldn’t carry the uncommitted work over: ${errorText(err)}`)
  }
}

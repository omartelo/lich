import type { Shape } from "../../catalog"
import type { prompts as en } from "../en/prompts"

export const prompts = {
  pullRequest: {
    conflicts: "拉取请求 #{number}（{branch}）与 {base} 存在合并冲突。请解决这些冲突。",
    checksUnnamed: {
      one: "拉取请求 #{number}（{branch}）的 CI 失败了。有 {count} 项检查未通过，请找出是哪些并修复。",
      other:
        "拉取请求 #{number}（{branch}）的 CI 失败了。有 {count} 项检查未通过，请找出是哪些并修复。",
    },
    checksListed: "拉取请求 #{number}（{branch}）的 CI 失败了。请修复以下检查：\n\n{checks}",
    checksOmitted: {
      one: "\n\n…另有 {count} 项失败的检查未列出。",
      other: "\n\n…另有 {count} 项失败的检查未列出。",
    },
    create:
      "分支 {branch} 还没有拉取请求。请用 `gh pr create` 创建一个：如果分支尚未推送到远程，先推送；根据提交和与基础分支的差异撰写标题和正文；如果仓库有拉取请求模板，请遵循该模板。",
  },
  delegate: {
    session: "委派给“{target}”会话：",
    newWorktree: "委派给一个新的工作树会话：",
    newWorktreeWithCommand:
      "委派给一个新的工作树会话（运行 `lich open --worktree <branch> --prompt <task>`）：",
  },
  reviewComments: {
    header: "审查评论：",
  },
  issue: {
    head: "GitHub issue #{number}，{title}\n{url}",
  },
} satisfies Shape<typeof en>

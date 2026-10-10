import type { Shape } from "../../catalog"
import type { git as en } from "../en/git"

export const git = {
  baseStatus: {
    behind: {
      one: "落后 {base} {count} 个提交",
      other: "落后 {base} {count} 个提交",
    },
    conflict: { one: "{count} 个文件有冲突", other: "{count} 个文件有冲突" },
  },
  fileSearch: {
    cutMatches: "{count}+ 处匹配",
    summary: "{matches}（{files}）",
    matches: { one: "{count} 处匹配", other: "{count} 处匹配" },
    cutNote: "仅显示前 {count} 个匹配行。请缩小搜索范围。",
    tooLarge: {
      one: "{count} 个超过 1 MB 的文件未被搜索。",
      other: "{count} 个超过 1 MB 的文件未被搜索。",
    },
  },
  fileTree: {
    hidden: "已隐藏：{names}。",
    cut: "此文件夹中的文件数超出了文件树能列出的数量。",
  },
  lastTurn: {
    saidPreviousTurn: "来自上一轮",
    noTurnWindow: "{name} 既不报告一轮的开始也不报告其结束，因此没有可划定的时间窗口。",
  },
  carry: {
    reused: "{name} 已存在，因此按现状检出，未带入未提交的工作。",
    failed: "无法带入未提交的工作：{error}",
  },
  codemirror: {
    expand: { one: "展开 {count} 行未更改内容", other: "展开 {count} 行未更改内容" },
    showLines: "显示第 {from}–{to} 行",
    revertTitle: "还原此更改",
    revert: "还原",
  },
} satisfies Shape<typeof en>

import type { Shape } from "../../catalog"
import type { palette as en } from "../en/palette"

export const palette = {
  commandPalette: {
    title: "命令面板",
    placeholder: "跳转到会话、项目或某条消息，或输入 > 执行操作…",
    searchLabel: "搜索会话和项目",
    resultsLabel: "结果",
    filterLabel: "筛选结果",
    forgetFailed: "无法忘记 {label}：{error}",
    resumeFailed: "无法恢复 {label}：{error}",
    actionUnavailable: "{label} 在此处不可用",
    showBeside: "在旁边显示",
    nothingClosed: "还没有关闭的内容",
    nothingClosedHint: "关闭的会话会保留在这里，包括它的分支、智能体和对话，直到它的工作树被移除。",
    noMatches: "没有与 {query} 匹配的结果",
    noMatchesInTab: "在{tab}中没有与 {query} 匹配的结果",
    shownOfTotal: "{shown} / {total}",
    tab: {
      All: "全部",
      Sessions: "会话",
      Projects: "项目",
      Messages: "消息",
      History: "历史",
    },
    checkoutGone: "检出已不存在",
    matches: { one: "{count} 处匹配", other: "{count} 处匹配" },
    sessions: { one: "{count} 个会话", other: "{count} 个会话" },
    relocate: "重新定位",
    reopen: "重新打开",
  },
} satisfies Shape<typeof en>

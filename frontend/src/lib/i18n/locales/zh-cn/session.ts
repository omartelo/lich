import type { Shape } from "../../catalog"
import type { session as en } from "../en/session"

export const session = {
  schedule: {
    choice15Minutes: "15 分钟",
    choice1Hour: "1 小时",
    choice4Hours: "4 小时",
    choiceTomorrow: "明天",
    countdown: "{amount}后",
    today: "今天 {clock}",
    weekdayAt: "{weekday} {clock}",
  },
  limitLine: {
    sessionWindow: "会话上限",
    weeklyWindow: "每周上限",
    usageWindow: "用量上限",
  },
  filter: {
    phase: {
      waiting: "等待中",
      running: "运行中",
      unread: "未读",
      idle: "空闲",
    },
    phaseJoiner: "或",
    noMatch: "没有会话。当前会话保持显示。",
    noMatchQuery: "没有与“{query}”匹配的会话。当前会话保持显示。",
    noMatchPhases: "没有{phases}的会话。当前会话保持显示。",
    noMatchPhasesQuery: "没有与“{query}”匹配的{phases}会话。当前会话保持显示。",
  },
  cost: {
    mixedModels: "此对话切换过模型，因此 lich 无法计价。",
    unpricedModel: "此模型暂无价格，lich 需要联网获取。",
  },
  handsOn: {
    detailTurn:
      "此会话被实际操作的时长，包括输入、汇报或运行一轮的时间。超过 15 分钟的间隔计为离开。",
    detailTool:
      "此会话被实际操作的时长，包括输入或汇报工具调用的时间。超过 15 分钟的间隔计为离开。",
  },
  spawnGate: {
    checkoutGone: "此会话的工作树已不存在，因此会话已关闭。请重新创建工作树以继续。",
    conversationGone: "之前的对话已不可用，正在启动新会话。",
  },
  fork: {
    unavailable: "{name} 不支持分叉，只能恢复。",
  },
  palette: {
    untitledConversation: "未命名对话",
    groupSessions: "会话",
    groupProjects: "项目",
    groupMessages: "消息",
    groupOpen: "已打开",
    groupClosed: "已关闭",
    groupClosedSessions: "已关闭的会话",
    groupOutsideLich: "lich 之外",
    indexing: { one: "正在索引 {count} 个会话", other: "正在索引 {count} 个会话" },
  },
} satisfies Shape<typeof en>

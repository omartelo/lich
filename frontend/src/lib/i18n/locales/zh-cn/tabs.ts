import type { Shape } from "../../catalog"
import type { tabs as en } from "../en/tabs"

export const tabs = {
  homeTab: {
    home: "主页",
  },
  notificationsButton: {
    title: "通知",
    titlePending: "通知，{count} 条待处理",
    caughtUp: "你已处理完所有通知",
    dismiss: "忽略 {name}",
  },
  openProjectMenu: {
    open: "打开项目",
    recent: "最近的项目",
    relocate: "重新定位",
    openFolder: "打开文件夹…",
    moreClosed: {
      one: "还有 {count} 个已关闭的项目，请在命令面板中搜索",
      other: "还有 {count} 个已关闭的项目，请在命令面板中搜索",
    },
  },
  projectTab: {
    close: "关闭 {name}",
  },
  projectTabs: {
    pullRequests: "拉取请求",
    settings: "设置",
    closeTitle: "会话仍在运行",
    closeBody: {
      one: "关闭 {name} 会中断一个正在进行一轮的会话。重新打开项目时可选择从每个会话中断处恢复，进行中的那一轮会丢失。",
      other:
        "关闭 {name} 会中断 {count} 个正在进行一轮的会话。重新打开项目时可选择从每个会话中断处恢复，进行中的那一轮会丢失。",
    },
    closeAnyway: "仍然关闭",
  },
} satisfies Shape<typeof en>

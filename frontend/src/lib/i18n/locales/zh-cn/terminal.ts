import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"

export const terminal = {
  dropHint: {
    attachTo: "附加到 {label}",
    confined: "位于检出目录之外时，会以副本形式传入",
    pasted: "其路径会粘贴到提示符处",
  },
  exitBanner: {
    restart: "重新启动",
    close: "关闭",
  },
  searchBar: {
    placeholder: "查找",
    label: "搜索终端",
    previous: "上一个匹配项",
    next: "下一个匹配项",
    close: "关闭搜索",
  },
  drop: {
    notAttached: "未附加：{files}",
  },
  host: {
    stopShowing: "不再显示 {name}",
    paneName: "{name} 窗格",
    rememberFailed: "无法记住该选择：{error}",
  },
  view: {
    restartFailed: "会话重新启动失败：{error}",
    startFailed: "会话启动失败：{error}",
    pasteSettingFailed: "无法读取是否展开长粘贴，因此本次只粘贴了一次：{error}",
  },
  sessionExit: {
    ended: "会话已结束",
    endedWithCode: "会话已结束，退出码 {code}",
  },
  dropFiles: {
    folderConfined: "无法移交沙箱会话检出目录之外的文件夹，请拖入文件",
    folderMissing: "在此会话或你的主目录下找不到该文件夹，请拖入文件",
  },
  copyToast: {
    copied: {
      one: "已复制 {count} 个字符到剪贴板",
      other: "已复制 {count} 个字符到剪贴板",
    },
  },
} satisfies Shape<typeof en>

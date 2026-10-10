import type { Shape } from "../../catalog"
import type { common as en } from "../en/common"

export const common = {
  count: {
    file: { one: "{count} 个文件", other: "{count} 个文件" },
    commit: { one: "{count} 个提交", other: "{count} 个提交" },
  },
  time: {
    justNow: "刚刚",
  },
  action: {
    cancel: "取消",
    save: "保存",
    create: "创建",
  },
  checkAgainButton: {
    check: "重新检查",
    checking: "正在检查…",
  },
  errorBoundary: {
    stoppedRendering: "{label} 已停止渲染",
    reload: "重新加载窗口",
    retry: "重试",
  },
  stepper: {
    default: "默认",
  },
  toolMissing: {
    notInstalled: "未安装 {label}",
    install: "安装 {bin}",
  },
  pickerDialog: {
    navigate: "导航",
    filter: "筛选",
    close: "关闭",
  },
  ui: {
    dialog: {
      close: "关闭",
    },
  },
} satisfies Shape<typeof en>

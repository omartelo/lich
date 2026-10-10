import type { Shape } from "../../catalog"
import type { env as en } from "../en/env"

export const env = {
  providerSummary: {
    installed: "已安装",
    installedCustomPath: "已安装 · 自定义路径",
    signedOut: "已登出",
    windowPercent: "{length} 窗口的 {percent}%",
    percentUsed: "已用 {percent}%",
  },
  quota: {
    tokenLogin: "令牌登录",
    weekShort: "周",
  },
  paths: {
    cwdUnknown: "工作目录未知 · 位于 {host} 内",
  },
  sandbox: {
    windowsReason: "lich 在 Windows 上没有沙箱后端",
    windowsAdvice: "无需安装任何东西，每个会话都直接在本机上运行。",
    macReason: "sandbox-exec 不可用",
    macAdvice:
      "macOS 自带 /usr/bin/sandbox-exec，所以缺少可用的 sandbox-exec 意味着系统已损坏，lich 无法修复。每个会话都直接在本机上运行。",
    linuxReason: "未安装 bubblewrap",
    linuxAdvice: "请安装 bubblewrap 并重新打开 lich。在此之前，每个会话都直接在本机上运行。",
    confinedMeans:
      "一个只包含智能体自身状态的空主目录，本机只读，只能写入其检出目录内部。网络保持开启。",
  },
  vcsTools: {
    gitWithout: "没有它，分支、差异和工作树都将为空。",
    ghWithout: "没有它，无法使用拉取请求、检查和 PR 检出。",
  },
  binary: {
    parkedProject: "{name} 覆盖已关闭",
    parkedProjectFallback: "项目",
    parkedGlobal: "全局覆盖已关闭",
    executable: "可执行",
    noSuchFile: "文件不存在",
    notOnPath: "不在 $PATH 中",
    notExecutable: "不可执行",
    homeNotExpanded: "~ 不会被展开",
    relativePath: "相对路径",
    detailHomeShortcut:
      "lich 直接启动该二进制文件，因此 ~ 按字面处理而不会被展开。请使用完整路径。",
    detailRelative:
      "相对路径会按每个会话自己的工作目录解析，因此每个会话指向的二进制文件各不相同。请使用完整路径。",
    detailBroken: "问题解决之前，会话无法启动。你也可以关闭或清除覆盖设置，回退到下一层的配置。",
  },
  commitIdentity: {
    noneLead: "此检出中没有 git 身份。",
    noneNote: "在设置 user.email 之前，提交将被拒绝。",
    landAsNamed: "提交将以 {name} 的身份记录",
    landAs: "提交将以以下身份记录",
    setLocal: "，在此仓库中设置，覆盖你的全局配置。",
    setGlobal: "，来自 git user.email，而非此账户。",
  },
} satisfies Shape<typeof en>

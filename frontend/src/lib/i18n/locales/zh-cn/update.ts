import type { Shape } from "../../catalog"
import type { update as en } from "../en/update"

export const update = {
  plugin: {
    incompatibleInstall:
      "{installs} 中的 lich 插件不受此 lich 支持。请安装 v{version}，即此 lich 支持的版本。",
    incompatibleUpdate:
      "{installs} 中的 lich 插件不受此 lich 支持。请更新 lich，或确认已联网以查找此 lich 支持的版本。",
  },
  progress: {
    updateFailed: "更新失败：{error}",
    installFailed: "安装失败：{error}",
    downloadFailed: "下载失败：{error}",
    downloadFailedAt: "下载在 {percent}% 时失败：{error}",
    bytesOf: "{received} / {total}",
  },
} satisfies Shape<typeof en>

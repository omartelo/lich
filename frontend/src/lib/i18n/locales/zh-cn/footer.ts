import type { Shape } from "../../catalog"
import type { footer as en } from "../en/footer"

export const footer = {
  item: {
    attach: { label: "附加文件", example: "附加文件" },
    files: { label: "文件资源管理器", example: "文件" },
    changes: { label: "更改" },
    pr: { label: "拉取请求" },
    checkout: { label: "分支" },
    path: { label: "工作目录" },
    model: { label: "模型", example: "模型" },
    context: { label: "上下文窗口" },
    plan: { label: "套餐用量" },
    cost: { label: "费用" },
    handsOn: { label: "实际操作时间" },
    clock: { label: "日期和时间" },
  },
} satisfies Shape<typeof en>

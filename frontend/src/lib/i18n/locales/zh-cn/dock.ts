import type { Shape } from "../../catalog"
import type { dock as en } from "../en/dock"

export const dock = {
  rightDock: {
    fileBrowser: "文件浏览器",
    reviewChanges: "审查更改",
    tabCode: "代码",
    tabReview: "审查",
    exitFullScreen: "退出全屏",
    fullScreen: "全屏",
    closePanel: "关闭面板",
    fileTree: "文件树",
    reviewPanel: "审查面板",
    resizePanel: "调整面板大小",
  },
  searchResults: {
    prompt: "搜索此检出中每个文件的文本",
    searching: "正在搜索…",
    noMatch: "没有文件包含“{query}”",
  },
  filesPanel: {
    searchPlaceholder: "在文件中搜索",
    filterPlaceholder: "按名称筛选",
    searchLabel: "在文件中搜索",
    filterLabel: "按名称筛选文件",
    matchesLabel: "输入框匹配的内容",
    byName: "名称",
    byText: "文本",
    readFailed: "无法读取此文件夹",
    loading: "正在加载…",
    noMatch: "没有匹配的文件",
    empty: "这里没有文件",
    backToTree: "返回文件树",
    readOnly: "只读",
  },
} satisfies Shape<typeof en>

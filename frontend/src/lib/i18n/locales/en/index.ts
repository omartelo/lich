import type { Paths } from "../../catalog"
import { common } from "./common"
import { diff } from "./diff"
import { dock } from "./dock"
import { palette } from "./palette"
import { prompts } from "./prompts"
import { pulls } from "./pulls"
import { settings } from "./settings"
import { shell } from "./shell"
import { sidebar } from "./sidebar"
import { terminal } from "./terminal"

// One namespace per component folder, named after it, plus `common`.
export const en = { common, diff, dock, palette, prompts, pulls, settings, shell, sidebar, terminal } as const

export type Messages = typeof en
export type MessageKey = Paths<Messages>

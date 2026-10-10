import type { Paths } from "../../catalog"
import { common } from "./common"
import { diff } from "./diff"
import { prompts } from "./prompts"
import { settings } from "./settings"
import { sidebar } from "./sidebar"

// One namespace per component folder, named after it, plus `common`.
export const en = { common, diff, prompts, settings, sidebar } as const

export type Messages = typeof en
export type MessageKey = Paths<Messages>

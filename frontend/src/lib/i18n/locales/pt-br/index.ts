import type { Shape } from "../../catalog"
import type { Messages } from "../en"
import { common } from "./common"
import { diff } from "./diff"
import { dock } from "./dock"
import { palette } from "./palette"
import { prompts } from "./prompts"
import { pulls } from "./pulls"
import { settings } from "./settings"
import { shell } from "./shell"
import { sidebar } from "./sidebar"
import { tabs } from "./tabs"
import { terminal } from "./terminal"

export const ptBR = {
  common,
  diff,
  dock,
  palette,
  prompts,
  pulls,
  settings,
  shell,
  sidebar,
  tabs,
  terminal,
} satisfies Shape<Messages>

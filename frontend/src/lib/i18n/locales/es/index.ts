import type { Shape } from "../../catalog"
import type { Messages } from "../en"
import { common } from "./common"
import { diff } from "./diff"
import { dock } from "./dock"
import { env } from "./env"
import { footer } from "./footer"
import { git } from "./git"
import { hotkeys } from "./hotkeys"
import { palette } from "./palette"
import { prompts } from "./prompts"
import { pulls } from "./pulls"
import { session } from "./session"
import { settings } from "./settings"
import { shell } from "./shell"
import { sidebar } from "./sidebar"
import { tabs } from "./tabs"
import { terminal } from "./terminal"
import { update } from "./update"

export const es = {
  common,
  diff,
  dock,
  env,
  footer,
  git,
  hotkeys,
  palette,
  prompts,
  pulls,
  session,
  settings,
  shell,
  sidebar,
  tabs,
  terminal,
  update,
} satisfies Shape<Messages>

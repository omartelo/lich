import type { Shape } from "../../catalog"
import type { Messages } from "../en"
import { common } from "./common"
import { diff } from "./diff"
import { prompts } from "./prompts"
import { pulls } from "./pulls"
import { settings } from "./settings"
import { sidebar } from "./sidebar"

export const ptBR = { common, diff, prompts, pulls, settings, sidebar } satisfies Shape<Messages>

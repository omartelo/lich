import type { Shape } from "../../catalog"
import type { Messages } from "../en"
import { common } from "./common"
import { diff } from "./diff"
import { settings } from "./settings"
import { sidebar } from "./sidebar"

export const ptBR = { common, diff, settings, sidebar } satisfies Shape<Messages>

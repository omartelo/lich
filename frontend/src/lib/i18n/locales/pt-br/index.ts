import type { Shape } from "../../catalog"
import type { Messages } from "../en"
import { common } from "./common"
import { settings } from "./settings"
import { sidebar } from "./sidebar"

export const ptBR = { common, settings, sidebar } satisfies Shape<Messages>

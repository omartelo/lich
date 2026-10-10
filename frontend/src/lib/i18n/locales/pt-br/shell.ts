import type { Shape } from "../../catalog"
import type { shell as en } from "../en/shell"

export const shell = {} satisfies Shape<typeof en>

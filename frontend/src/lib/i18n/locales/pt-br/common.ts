import type { Shape } from "../../catalog"
import type { common as en } from "../en/common"

export const common = {
  count: {
    file: { one: "{count} arquivo", other: "{count} arquivos" },
    commit: { one: "{count} commit", other: "{count} commits" },
  },
  time: {
    justNow: "agora mesmo",
  },
  action: {
    cancel: "Cancelar",
    save: "Salvar",
    create: "Criar",
  },
} satisfies Shape<typeof en>

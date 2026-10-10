import type { Shape } from "../../catalog"
import type { footer as en } from "../en/footer"

export const footer = {
  item: {
    attach: { label: "Anexar arquivo", example: "Anexar arquivo" },
    files: { label: "Explorador de arquivos", example: "Arquivos" },
    changes: { label: "Alterações" },
    pr: { label: "Pull request" },
    checkout: { label: "Branch" },
    path: { label: "Diretório de trabalho" },
    model: { label: "Modelo", example: "Modelo" },
    context: { label: "Janela de contexto" },
    plan: { label: "Uso do plano" },
    cost: { label: "Custo" },
    handsOn: { label: "Tempo de uso ativo" },
    clock: { label: "Data e hora" },
  },
} satisfies Shape<typeof en>

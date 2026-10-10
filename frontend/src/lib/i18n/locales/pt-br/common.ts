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
  checkAgainButton: {
    check: "Verificar novamente",
    checking: "Verificando…",
  },
  errorBoundary: {
    stoppedRendering: "{label} parou de renderizar",
    reload: "Recarregar a janela",
    retry: "Tentar novamente",
  },
  stepper: {
    reset: "Redefinir {name}",
    default: "Padrão",
  },
  toolMissing: {
    notInstalled: "{label} não está instalado",
    install: "Instalar {bin}",
  },
  pickerDialog: {
    navigate: "navegar",
    filter: "filtrar",
    close: "fechar",
  },
  ui: {
    dialog: {
      close: "Fechar",
    },
  },
} satisfies Shape<typeof en>

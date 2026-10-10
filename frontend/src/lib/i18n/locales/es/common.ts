import type { Shape } from "../../catalog"
import type { common as en } from "../en/common"

export const common = {
  count: {
    file: { one: "{count} archivo", other: "{count} archivos" },
    commit: { one: "{count} commit", other: "{count} commits" },
  },
  time: {
    justNow: "ahora mismo",
  },
  action: {
    cancel: "Cancelar",
    save: "Guardar",
    create: "Crear",
  },
  checkAgainButton: {
    check: "Comprobar de nuevo",
    checking: "Comprobando…",
  },
  errorBoundary: {
    stoppedRendering: "{label} dejó de mostrarse",
    reload: "Recargar la ventana",
    retry: "Intentar de nuevo",
  },
  stepper: {
    default: "Predeterminado",
  },
  toolMissing: {
    notInstalled: "{label} no está instalado",
    install: "Instalar {bin}",
  },
  pickerDialog: {
    navigate: "navegar",
    filter: "filtrar",
    close: "cerrar",
  },
  ui: {
    dialog: {
      close: "Cerrar",
    },
  },
} satisfies Shape<typeof en>

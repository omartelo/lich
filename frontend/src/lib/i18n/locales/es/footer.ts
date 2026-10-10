import type { Shape } from "../../catalog"
import type { footer as en } from "../en/footer"

export const footer = {
  item: {
    attach: { label: "Adjuntar archivo", example: "Adjuntar archivo" },
    files: { label: "Explorador de archivos", example: "Archivos" },
    changes: { label: "Cambios" },
    pr: { label: "Pull request" },
    checkout: { label: "Rama" },
    path: { label: "Directorio de trabajo" },
    model: { label: "Modelo", example: "Modelo" },
    context: { label: "Ventana de contexto" },
    plan: { label: "Uso del plan" },
    cost: { label: "Costo" },
    handsOn: { label: "Tiempo activo" },
    clock: { label: "Fecha y hora" },
  },
} satisfies Shape<typeof en>

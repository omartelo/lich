import type { Shape } from "../../catalog"
import type { tabs as en } from "../en/tabs"

export const tabs = {
  homeTab: {
    home: "Inicio",
  },
  notificationsButton: {
    title: "Notificaciones",
    titlePending: "Notificaciones, {count} pendientes",
    caughtUp: "Estás al día",
    dismiss: "Descartar {name}",
  },
  openProjectMenu: {
    open: "Abrir proyecto",
    recent: "Proyectos recientes",
    relocate: "reubicar",
    openFolder: "Abrir carpeta…",
    moreClosed: {
      one: "{count} proyecto cerrado más: busca en la paleta",
      other: "{count} proyectos cerrados más: busca en la paleta",
    },
  },
  projectTab: {
    close: "Cerrar {name}",
  },
  projectTabs: {
    pullRequests: "Pull requests",
    settings: "Ajustes",
    closeTitle: "Hay sesiones en ejecución",
    closeBody: {
      one: "Cerrar {name} detiene una sesión a mitad de un turno. Al reabrir el proyecto se ofrece reanudar donde se quedó cada una; el turno en curso se pierde.",
      other:
        "Cerrar {name} detiene {count} sesiones a mitad de un turno. Al reabrir el proyecto se ofrece reanudar donde se quedó cada una; el turno en curso se pierde.",
    },
    closeAnyway: "Cerrar de todos modos",
  },
} satisfies Shape<typeof en>

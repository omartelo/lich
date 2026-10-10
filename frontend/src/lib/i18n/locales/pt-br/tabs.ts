import type { Shape } from "../../catalog"
import type { tabs as en } from "../en/tabs"

export const tabs = {
  homeTab: {
    home: "Início",
  },
  notificationsButton: {
    title: "Notificações",
    titlePending: "Notificações, {count} pendentes",
    caughtUp: "Tudo em dia",
    dismiss: "Dispensar {name}",
  },
  openProjectMenu: {
    open: "Abrir projeto",
    recent: "Projetos recentes",
    relocate: "relocalizar",
    openFolder: "Abrir pasta…",
    moreClosed: {
      one: "Mais {count} projeto fechado: busque na paleta",
      other: "Mais {count} projetos fechados: busque na paleta",
    },
  },
  projectTab: {
    close: "Fechar {name}",
  },
  projectTabs: {
    pullRequests: "Pull requests",
    settings: "Configurações",
    closeTitle: "Há sessões em execução",
    closeBody: {
      one: "Fechar {name} interrompe uma sessão no meio do turno. Ao reabrir o projeto, você pode retomar de onde cada uma parou; o turno em andamento é perdido.",
      other:
        "Fechar {name} interrompe {count} sessões no meio do turno. Ao reabrir o projeto, você pode retomar de onde cada uma parou; o turno em andamento é perdido.",
    },
    closeAnyway: "Fechar mesmo assim",
  },
} satisfies Shape<typeof en>

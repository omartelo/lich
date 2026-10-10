import type { Shape } from "../../catalog"
import type { palette as en } from "../en/palette"

export const palette = {
  commandPalette: {
    title: "Paleta de comandos",
    placeholder: "Ve a una sesión, un proyecto o algo que se dijo, o escribe > para ver acciones…",
    searchLabel: "Buscar sesiones y proyectos",
    resultsLabel: "Resultados",
    filterLabel: "Filtrar resultados",
    forgetFailed: "No se pudo olvidar {label}: {error}",
    resumeFailed: "No se pudo reanudar {label}: {error}",
    actionUnavailable: "{label} no está disponible aquí",
    nothingClosed: "Todavía no hay nada cerrado",
    nothingClosedHint:
      "Cierra una sesión y espera aquí, con su rama, su agente y su conversación, hasta que se elimine su worktree.",
    noMatches: "Sin coincidencias para {query}",
    noMatchesInTab: "Sin coincidencias para {query} en {tab}",
    shownOfTotal: "{shown} de {total}",
    tab: {
      All: "Todo",
      Sessions: "Sesiones",
      Projects: "Proyectos",
      Messages: "Mensajes",
      History: "Historial",
    },
    checkoutGone: "checkout desaparecido",
    matches: { one: "{count} coincidencia", other: "{count} coincidencias" },
    sessions: { one: "{count} sesión", other: "{count} sesiones" },
    relocate: "reubicar",
    reopen: "reabrir",
  },
} satisfies Shape<typeof en>

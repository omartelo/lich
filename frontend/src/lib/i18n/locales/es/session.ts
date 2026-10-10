import type { Shape } from "../../catalog"
import type { session as en } from "../en/session"

export const session = {
  schedule: {
    choice15Minutes: "15 min",
    choice1Hour: "1 hora",
    choice4Hours: "4 horas",
    choiceTomorrow: "Mañana",
    countdown: "en {amount}",
    today: "hoy {clock}",
    weekdayAt: "{weekday} {clock}",
  },
  limitLine: {
    sessionWindow: "Límite de sesión",
    weeklyWindow: "Límite semanal",
    usageWindow: "Límite de uso",
  },
  filter: {
    phase: {
      waiting: "en espera",
      running: "en ejecución",
      unread: "sin leer",
      idle: "inactivas",
    },
    phaseJoiner: " o ",
    noMatch: "No hay sesiones. La sesión activa se mantiene.",
    noMatchQuery: "Ninguna sesión coincide con “{query}”. La sesión activa se mantiene.",
    noMatchPhases: "No hay sesiones {phases}. La sesión activa se mantiene.",
    noMatchPhasesQuery:
      "Ninguna sesión {phases} coincide con “{query}”. La sesión activa se mantiene.",
  },
  cost: {
    mixedModels: "Esta conversación cambió de modelo, así que lich no puede calcular su precio.",
    unpricedModel: "Aún no hay precio para este modelo: lich necesita la red para obtenerlo.",
  },
  handsOn: {
    detailTurn:
      "Cuánto tiempo se ha trabajado en esta sesión: escribiéndole, informando o ejecutando un turno. Una pausa de más de 15 minutos cuenta como tiempo de ausencia.",
    detailTool:
      "Cuánto tiempo se ha trabajado en esta sesión: escribiéndole o informando de una llamada a una herramienta. Una pausa de más de 15 minutos cuenta como tiempo de ausencia.",
  },
  spawnGate: {
    checkoutGone:
      "La worktree de esta sesión ya no existe, así que la sesión se cerró. Vuelve a crear la worktree para retomarla.",
    conversationGone: "La conversación anterior ya no está disponible: se inicia una nueva sesión.",
  },
  fork: {
    unavailable: "{name} no conserva forks; solo se puede reanudar.",
  },
  palette: {
    untitledConversation: "Conversación sin título",
    groupSessions: "Sesiones",
    groupProjects: "Proyectos",
    groupMessages: "Mensajes",
    groupOpen: "Abiertas",
    groupClosed: "Cerradas",
    groupClosedSessions: "Sesiones cerradas",
    groupOutsideLich: "Fuera de lich",
    indexing: { one: "indexando {count} sesión", other: "indexando {count} sesiones" },
  },
} satisfies Shape<typeof en>

import type { Shape } from "../../catalog"
import type { session as en } from "../en/session"

export const session = {
  schedule: {
    choice15Minutes: "15 min",
    choice1Hour: "1 hora",
    choice4Hours: "4 horas",
    choiceTomorrow: "Amanhã",
    countdown: "em {amount}",
    today: "hoje {clock}",
    weekdayAt: "{weekday} {clock}",
  },
  limitLine: {
    sessionWindow: "Limite da sessão",
    weeklyWindow: "Limite semanal",
    usageWindow: "Limite de uso",
  },
  filter: {
    phase: {
      waiting: "aguardando",
      running: "em execução",
      unread: "não lidas",
      idle: "ociosas",
    },
    phaseJoiner: " ou ",
    noMatch: "Nenhuma sessão. A sessão ativa continua.",
    noMatchQuery: "Nenhuma sessão corresponde a “{query}”. A sessão ativa continua.",
    noMatchPhases: "Nenhuma sessão {phases}. A sessão ativa continua.",
    noMatchPhasesQuery: "Nenhuma sessão {phases} corresponde a “{query}”. A sessão ativa continua.",
  },
  cost: {
    mixedModels: "Esta conversa trocou de modelo, então o lich não consegue calcular o preço.",
    unpricedModel: "Ainda não há preço para este modelo. O lich precisa da rede para buscar um.",
  },
  handsOn: {
    detailTurn:
      "Há quanto tempo esta sessão está em uso: digitando nela, recebendo relatórios ou executando um turno. Uma pausa maior que 15 minutos conta como tempo ausente.",
    detailTool:
      "Há quanto tempo esta sessão está em uso: digitando nela ou recebendo o relatório de uma chamada de ferramenta. Uma pausa maior que 15 minutos conta como tempo ausente.",
  },
  spawnGate: {
    checkoutGone:
      "O worktree desta sessão não existe mais, então a sessão foi fechada. Recrie o worktree para retomá-la.",
    conversationGone: "A conversa anterior não está mais disponível. Iniciando uma nova sessão.",
  },
  fork: {
    unavailable: "{name} não tem fork; só é possível retomar.",
  },
  palette: {
    untitledConversation: "Conversa sem título",
    groupSessions: "Sessões",
    groupProjects: "Projetos",
    groupMessages: "Mensagens",
    groupOpen: "Abertos",
    groupClosed: "Fechados",
    groupClosedSessions: "Sessões fechadas",
    groupOutsideLich: "Fora do lich",
    indexing: { one: "indexando {count} sessão", other: "indexando {count} sessões" },
  },
} satisfies Shape<typeof en>

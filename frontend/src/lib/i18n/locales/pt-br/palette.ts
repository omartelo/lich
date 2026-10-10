import type { Shape } from "../../catalog"
import type { palette as en } from "../en/palette"

export const palette = {
  commandPalette: {
    title: "Paleta de comandos",
    placeholder: "Vá para uma sessão, projeto ou algo que foi dito, ou digite > para ações…",
    searchLabel: "Buscar sessões e projetos",
    resultsLabel: "Resultados",
    filterLabel: "Filtrar resultados",
    forgetFailed: "Não foi possível esquecer {label}: {error}",
    resumeFailed: "Não foi possível retomar {label}: {error}",
    actionUnavailable: "{label} não está disponível aqui",
    showBeside: "mostrar ao lado",
    nothingClosed: "Nada fechado ainda",
    nothingClosedHint:
      "Feche uma sessão e ela espera aqui, com sua branch, seu agente e sua conversa, até que o worktree seja removido.",
    noMatches: "Nenhum resultado para {query}",
    noMatchesInTab: "Nenhum resultado para {query} em {tab}",
    shownOfTotal: "{shown} de {total}",
    tab: {
      All: "Tudo",
      Sessions: "Sessões",
      Projects: "Projetos",
      Messages: "Mensagens",
      History: "Histórico",
    },
    checkoutGone: "checkout removido",
    matches: { one: "{count} ocorrência", other: "{count} ocorrências" },
    sessions: { one: "{count} sessão", other: "{count} sessões" },
    relocate: "relocalizar",
    reopen: "reabrir",
  },
} satisfies Shape<typeof en>

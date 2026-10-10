import type { Shape } from "../../catalog"
import type { git as en } from "../en/git"

export const git = {
  baseStatus: {
    behind: {
      one: "{count} commit atrás de {base}",
      other: "{count} commits atrás de {base}",
    },
    conflict: { one: "Conflitos em {count} arquivo", other: "Conflitos em {count} arquivos" },
  },
  fileSearch: {
    cutMatches: "{count}+ resultados",
    summary: "{matches} em {files}",
    matches: { one: "{count} resultado", other: "{count} resultados" },
    cutNote: "Mostrando as primeiras {count} linhas encontradas. Restrinja a busca.",
    tooLarge: {
      one: "{count} arquivo acima de 1 MB não foi pesquisado.",
      other: "{count} arquivos acima de 1 MB não foram pesquisados.",
    },
  },
  fileTree: {
    hidden: "Ocultos: {names}.",
    cut: "Esta pasta tem mais arquivos do que a árvore consegue listar.",
  },
  lastTurn: {
    saidPreviousTurn: "do turno anterior",
    noTurnWindow:
      "{name} não informa o início nem o fim de um turno, então não há janela para delimitar.",
  },
  carry: {
    reused:
      "{name} já existia, então foi aberto como estava. O trabalho não commitado não foi transferido.",
    failed: "Não foi possível transferir o trabalho não commitado: {error}",
  },
  codemirror: {
    expand: {
      one: "Expandir {count} linha sem alteração",
      other: "Expandir {count} linhas sem alteração",
    },
    showLines: "Mostrar linhas {from}–{to}",
    revertTitle: "Reverter esta alteração",
    revert: "Reverter",
  },
} satisfies Shape<typeof en>

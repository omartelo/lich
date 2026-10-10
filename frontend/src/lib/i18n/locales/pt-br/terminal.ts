import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"

export const terminal = {
  dropHint: {
    attachTo: "Anexar a {label}",
    confined: "Fora do checkout, chega como uma cópia",
    pasted: "O caminho é colado no prompt",
  },
  exitBanner: {
    restart: "Reiniciar",
    close: "Fechar",
  },
  searchBar: {
    placeholder: "Buscar",
    label: "Pesquisar no terminal",
    previous: "Resultado anterior",
    next: "Próximo resultado",
    close: "Fechar pesquisa",
  },
  drop: {
    notAttached: "Não anexado: {files}",
  },
  host: {
    stopShowing: "Parar de mostrar {name}",
    paneName: "O painel {name}",
    rememberFailed: "Não foi possível lembrar a escolha: {error}",
    showBeside: "Mostrar uma sessão ao lado desta",
    noRoom: "Não cabe outro painel",
  },
  showBesidePicker: {
    title: "Mostrar ao lado",
    placeholder: "Buscar sessões para mostrar ao lado…",
    search: "Buscar sessões para mostrar ao lado",
    results: "Sessões que podem ser mostradas ao lado",
    pick: "mostrar",
    noMatch: "Nenhuma sessão corresponde a {query}",
    onWall: "em {group}",
  },
  view: {
    restartFailed: "Falha ao reiniciar a sessão: {error}",
    startFailed: "Falha ao iniciar a sessão: {error}",
    pasteSettingFailed:
      "Não deu para ler se o paste longo deve ser expandido, então este foi colado uma vez: {error}",
  },
  sessionExit: {
    ended: "Sessão encerrada",
    endedWithCode: "Sessão encerrada com código {code}",
  },
  dropFiles: {
    folderConfined:
      "pastas fora do checkout desta sessão em sandbox não podem ser entregues; solte arquivos",
    folderMissing: "pasta não encontrada nesta sessão nem na sua home; solte arquivos",
  },
  copyToast: {
    copied: {
      one: "{count} caractere copiado para a área de transferência",
      other: "{count} caracteres copiados para a área de transferência",
    },
  },
} satisfies Shape<typeof en>

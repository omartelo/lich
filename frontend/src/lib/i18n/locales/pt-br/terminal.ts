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

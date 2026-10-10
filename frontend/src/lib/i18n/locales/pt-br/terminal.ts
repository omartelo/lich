import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"

export const terminal = {
  dropHint: {
    attachTo: "Anexar a {label}",
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
  },
} satisfies Shape<typeof en>

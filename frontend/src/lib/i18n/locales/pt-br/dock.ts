import type { Shape } from "../../catalog"
import type { dock as en } from "../en/dock"

export const dock = {
  rightDock: {
    fileBrowser: "Navegador de arquivos",
    reviewChanges: "Revisar alterações",
    tabCode: "Código",
    tabReview: "Revisão",
    exitFullScreen: "Sair da tela cheia",
    fullScreen: "Tela cheia",
    closePanel: "Fechar painel",
    fileTree: "A árvore de arquivos",
    reviewPanel: "O painel de revisão",
    resizePanel: "Redimensionar painel",
  },
  searchResults: {
    prompt: "Busque no texto de todos os arquivos deste checkout",
    searching: "Buscando…",
    noMatch: "Nenhum arquivo contém “{query}”",
  },
  filesPanel: {
    searchPlaceholder: "Buscar nos arquivos",
    filterPlaceholder: "Filtrar por nome",
    searchLabel: "Buscar nos arquivos",
    filterLabel: "Filtrar arquivos por nome",
    matchesLabel: "O que o campo compara",
    byName: "Nome",
    byText: "Texto",
    readFailed: "Não foi possível ler esta pasta",
    loading: "Carregando…",
    noMatch: "Nenhum arquivo corresponde",
    empty: "Nenhum arquivo aqui",
    backToTree: "Voltar à árvore de arquivos",
    readOnly: "somente leitura",
  },
} satisfies Shape<typeof en>

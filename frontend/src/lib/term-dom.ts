// ghostty-web é dono de exatamente um <canvas> + um <textarea> escondido dentro
// do seu container contenteditable. Qualquer outro nó é conteúdo editável que o
// WebKitGTK inseriu por fora do guard de beforeinput do ghostty (paste de seleção
// primária por clique-do-meio no X11, drag-drop) — ele empurra o canvas em fluxo
// e fica selecionável. É parasita: remover.
const OWNED_TERMINAL_NODES = new Set(["CANVAS", "TEXTAREA"])

export function isStrayTerminalChild(node: Pick<Node, "nodeName">): boolean {
  return !OWNED_TERMINAL_NODES.has(node.nodeName)
}

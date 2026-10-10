import type { Shape } from "../../catalog"
import type { env as en } from "../en/env"

export const env = {
  providerSummary: {
    installed: "Instalado",
    installedCustomPath: "Instalado · caminho personalizado",
    signedOut: "Sem login",
    windowPercent: "{percent}% da janela de {length}",
    percentUsed: "{percent}% usado",
  },
  quota: {
    weekShort: "sem",
    tokenLogin: "Login por token",
  },
  paths: {
    cwdUnknown: "diretório desconhecido · dentro de {host}",
  },
  sandbox: {
    windowsReason: "o lich não tem backend de sandbox no Windows",
    windowsAdvice: "Não há nada para instalar. Toda sessão roda direto na máquina.",
    macReason: "o sandbox-exec não está disponível",
    macAdvice:
      "O macOS já traz o /usr/bin/sandbox-exec, então uma máquina sem ele funcionando está quebrada de um jeito que o lich não consegue consertar. Toda sessão roda direto na máquina.",
    linuxReason: "o bubblewrap não está instalado",
    linuxAdvice:
      "Instale o bubblewrap e reabra o lich. Até lá, toda sessão roda direto na máquina.",
    confinedMeans:
      "Uma home vazia com apenas o estado do próprio agente, a máquina somente leitura e escrita só dentro do seu checkout. A rede continua ligada.",
  },
  vcsTools: {
    gitWithout: "Branches, diffs e worktrees ficam vazios sem ele.",
    ghWithout: "Pull requests, checks e checkouts de PR ficam indisponíveis sem ele.",
  },
  binary: {
    parkedProject: "override de {name} desligado",
    parkedProjectFallback: "projeto",
    parkedGlobal: "override global desligado",
    executable: "executável",
    noSuchFile: "arquivo não encontrado",
    notOnPath: "fora do $PATH",
    notExecutable: "não executável",
    homeNotExpanded: "~ não é expandido",
    relativePath: "caminho relativo",
    detailHomeShortcut:
      "O lich executa o binário diretamente, então o ~ é tomado literalmente em vez de expandido. Use o caminho completo.",
    detailRelative:
      "Um caminho relativo é resolvido a partir do diretório de trabalho de cada sessão, então aponta para um binário diferente por sessão. Use um caminho completo.",
    detailBroken:
      "As sessões não iniciam até isso ser corrigido. Um override também pode ser desligado ou limpo, voltando para a camada abaixo.",
  },
  commitIdentity: {
    noneLead: "Nenhuma identidade git neste checkout.",
    noneNote: "Os commits serão recusados até que user.email seja definido.",
    landAsNamed: "Os commits saem como {name}",
    landAs: "Os commits saem como",
    setLocal: "— definido neste repositório, sobrepondo o seu global.",
    setGlobal: "— user.email do git, não desta conta.",
  },
} satisfies Shape<typeof en>

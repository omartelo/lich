import type { Shape } from "../../catalog"
import type { shell as en } from "../en/shell"

export const shell = {
  agentPluginGate: {
    updateAvailable: "O plugin do lich {version} está disponível",
    update: "Atualizar",
    later: "Depois",
    installVersion: "Instalar v{version}",
    updating: "Atualizando o plugin do lich…",
    updated: "Plugin atualizado. Reinicie suas sessões para aplicar.",
    updateFailed: "Falha na atualização",
    installed: "Plugin instalado. Reinicie suas sessões para aplicar.",
    codexTrust: "Codex: rode /hooks em uma sessão do Codex para confiar nos hooks do plugin.",
    installFailed: "Falha na instalação: {error}",
    title: "Ativar a integração com agentes",
    description:
      "O plugin do lich informa ao app o status e os títulos das suas sessões, atualiza o status do git delas e permite que uma sessão restaurada retome a conversa. Instale-o nas CLIs que você usa.",
    needsHooks: "precisa de /hooks para confiar nele",
    dontAskAgain: "Não perguntar de novo",
    notNow: "Agora não",
    installing: "Instalando…",
    install: "Instalar",
  },
  appUpdateGate: {
    available: "O lich {version} está disponível",
    updateAndInstall: "Atualizar e instalar",
    later: "Depois",
    updated: "lich atualizado para {version}",
    retry: "Tentar de novo",
    restart: "Reiniciar",
    restartFailed: "Falha ao reiniciar: {error}",
    install: "Instalar",
    viewRelease: "Ver versão",
  },
  confirmDialog: {
    cancel: "Cancelar",
  },
  emptySessions: {
    title: "Nenhuma sessão aberta",
    description: "Abra uma sessão para começar a trabalhar neste projeto.",
    noAgent:
      "Nenhum agente encontrado no PATH, então isto abre um terminal. Defina um em Configurações › Provedores.",
    newTerminal: "Novo terminal",
    newSession: "Nova sessão",
  },
  fileTree: {
    openInEditor: "Abrir no editor",
    expandAll: "Expandir tudo",
    collapseAll: "Recolher tudo",
  },
  footerBar: {
    attachFailed: "Não foi possível anexar o arquivo",
    attachFile: "Anexar arquivo",
    browseCode: "Navegar pelo código",
    reviewChanges: "Revisar alterações",
    viewPullRequest: "Ver pull request",
    pullRequest: "PR #{number}",
    leftFooter: "Rodapé esquerdo",
    rightFooter: "Rodapé direito",
  },
  footerSession: {
    contextWindow: "Janela de contexto",
    contextTokens: "{tokens} / {window} tokens",
    costUnavailable: "Custo indisponível",
    sessionCost: "Custo da sessão",
    costOfBudget: "{cost} de {budget} de orçamento",
    costNote:
      "Custo da API nas conversas desta sessão. As cobranças da assinatura podem ser diferentes.",
    handsOnTime: "Tempo de interação",
    handsOnFailed: "Não foi possível ler o tempo de interação.",
  },
  gitMissingGate: {
    title: "O git não está instalado",
    description:
      "O lich lê cada branch, diff e worktree pelo git. As sessões continuam funcionando sem ele, mas as telas de controle de versão ficam vazias.",
    notNow: "Agora não",
    installGit: "Instalar o git",
  },
  home: {
    title: "Nenhum projeto aberto",
    description: "Abra uma pasta para iniciar uma sessão de terminal.",
    openProject: "Abrir projeto",
  },
  notificationsOptIn: {
    title: "Avisar quando uma sessão precisar de você?",
    description:
      "Uma sessão acabou de pedir sua atenção enquanto você estava em outro lugar. O lich pode mostrar uma notificação na área de trabalho quando isso acontecer, com o nome da sessão e do projeto, para você iniciar uma tarefa e se afastar da janela. Sessões que você está olhando nunca notificam. Você pode mudar isso quando quiser em Configurações › Notificações.",
    noThanks: "Não, obrigado",
    notifyMe: "Me avisar",
  },
  paneSeams: {
    resizeColumns: "Redimensionar as colunas",
    resizeRows: "Redimensionar as linhas",
  },
  patchNotesDialog: {
    title: "Novidades do lich {version}",
    viewChangelog: "Ver o changelog completo",
    gotIt: "Entendi",
  },
  planQuota: {
    planUsage: "Uso do plano",
    locked: "Bloqueado",
  },
  providerSetupDialog: {
    chooseTitle: "Escolha seus agentes",
    noneTitle: "Nenhum agente de código encontrado",
    chooseDescription:
      "O lich encontrou estes na sua máquina. Ative os que você usa; os demais ficam fora do menu Nova sessão.",
    noneDescription:
      "O lich executa os agentes já instalados na sua máquina e não encontrou nenhum. Instale um e verifique de novo.",
    changeLater: "Você pode mudar isso quando quiser em Configurações › Provedores.",
    pointToBinary:
      "Já tem um instalado? Aponte o lich para o binário dele em Configurações › Provedores.",
    continue: "Continuar",
  },
  quotaGauge: {
    aheadOfPace: "Acima do ritmo",
    aheadTooltip: "Gastando acima do ritmo desta janela.",
    locked: "Bloqueado",
  },
  resumeSessionDialog: {
    title: "Retomar a sessão anterior?",
    description:
      "{label} deixou uma conversa para trás ({id}). Retome para continuar a conversa de onde parou, ou inicie uma nova para começar do zero.",
    alwaysFor: "Sempre fazer isso para {provider}",
    changeInSettings: "· mude em Configurações › Provedores",
    thisProvider: "este provedor",
    startNew: "Iniciar nova",
    resume: "Retomar",
  },
  shortcutsOverlay: {
    title: "Atalhos de teclado",
    rebind: "Reatribua em Configurações › Atalhos",
    close: "fechar",
  },
  quitDialog: {
    title: "Encerrar o lich?",
    description:
      "Todas as sessões terminam junto. Fechar a janela, em vez disso, as mantém rodando em segundo plano.",
    working: {
      one: "{count} sessão está no meio de um turno e o perde:",
      other: "{count} sessões estão no meio de um turno e o perdem:",
    },
    confirm: "Encerrar o lich",
    failed: "Não foi possível encerrar o lich",
  },
  uncleanExitGate: {
    title: "A execução anterior do lich terminou de forma inesperada",
    description: "Suas sessões foram restauradas, mas o que elas estavam executando parou junto.",
    openLogFolder: "Abrir pasta de logs",
    opening: "Abrindo a pasta de logs…",
    opened: "Pasta de logs aberta.",
    openFailed: "Não foi possível abrir a pasta de logs",
    dismiss: "Dispensar",
  },
  updateProgressToast: {
    downloading: "Baixando o lich {version}",
    installing: "Instalando o lich {version}…",
    reopens: "O lich fecha e reabre sozinho.",
  },
  app: {
    stage: "A área principal",
    reloadStage: "Recarregar a área principal",
    screen: "Esta tela",
    panel: "O painel",
  },
  projects: {
    relocated: "{name} agora abre {path}",
    relocateFailed: "Falha ao realocar: {error}",
    noLongerAvailable: "{label} não está mais disponível para retomar",
    bringBackFailed: "Não foi possível trazer {label} de volta",
    closed: "Sessão fechada: {label}",
    undo: "Desfazer",
    entrypointSet: "Entrypoint definido",
    entrypointSetDescription: "Executa na próxima vez que este terminal iniciar.",
    entrypointCleared: "Entrypoint removido",
    entrypointClearedDescription: "Este terminal volta a iniciar um shell simples.",
    entrypointFailed: "Não foi possível salvar o entrypoint: {error}",
    scheduled: "Agendado {when}",
    scheduledNow: "Agendado para agora",
    scheduleCleared: "Agendamento removido",
    scheduleFailed: "Não foi possível agendar: {error}",
  },
  projectEvents: {
    unlabeledSession: "Uma sessão",
    scheduleLost: "Prompt agendado perdido com {label}",
    scheduleDue: "Previsto {when}: {prompt}",
    relayAnswered: "{target} respondeu na própria sessão",
    relayDetail: "Terminou sem responder pelo lich. Abra para ler a resposta.",
    open: "Abrir",
    needsInput: "{label} precisa da sua atenção",
    finished: "{label} terminou o trabalho",
  },
} satisfies Shape<typeof en>

package prompt

var ptBR = Catalog{
	isOne: isOnePortuguese,

	BriefingIntro: "Você está rodando dentro do lich, que roda sessões de agentes de código lado a lado e " +
		"pode abrir outras ao lado desta: cada uma é um card que o usuário acompanha e pode assumir " +
		"no meio da tarefa, na própria git worktree quando o trabalho precisa de um checkout próprio. ",
	BriefingCards: "Aqui a sua própria ferramenta Agent as abre: um subagente general-purpose roda como " +
		"um desses cards, neste checkout a menos que você peça isolation 'worktree', em segundo " +
		"plano, e o relatório dele volta para você sozinho. Distribua o trabalho com ela. Abra uma " +
		"sessão você mesmo só para o que um subagente não pode ser: outro tipo de agente, uma " +
		"branch que o usuário nomeou, ou um trabalho que precisa sobreviver a esta sessão. ",
	BriefingNative: "Esta sessão é ela mesma um card de subagente que outra sessão abriu, e aqui a sua " +
		"própria ferramenta Agent roda um subagente dentro desta sessão em vez de como card. " +
		"Distribua o trabalho com ela, não com novas sessões do lich: a sua mensagem final é o " +
		"relatório que a sessão que abriu você está esperando.",
	BriefingSessions: "Quando o trabalho deve ser distribuído (várias tarefas ao mesmo tempo, uma por " +
		"branch ou checkout), são essas sessões que você abre, não os subagentes do seu próprio " +
		"harness: um subagente não tem checkout, nem card, nem nada que o usuário possa guiar ou " +
		"retomar. ",
	BriefingCommandTools: "As ferramentas do lich na sua lista abrem uma e entregam a tarefa a ela.",
	BriefingCommandCLI: "Abra uma com `lich open --worktree BRANCH --prompt 'a tarefa'`, que abre a " +
		"sessão e entrega a tarefa a ela em um só comando.",

	RelayMessage:  "[lich] %[1]s, não do seu próprio prompt.\n\n%[2]s\n\n%[3]s",
	OriginCLI:     "Mensagem repassada pela linha de comando do lich",
	OriginSession: "Mensagem da sessão %q",
	ReplyCommand:  "  \"$LICH_BIN\" reply %s \"<sua resposta>\"",
	ReplyRouteCLI: "Quando tiver uma resposta, envie de volta rodando:\n%s",
	ReplyRouteTools: "Quando tiver uma resposta, envie de volta com a ferramenta do lich `%[1]s` " +
		"(ticket %[2]s), ou rodando:\n%[3]s",
	ReplyOnlyWayBack: "Esse ticket é o único caminho de volta: quem pediu está bloqueado nele e não " +
		"está lendo mais nada. Não responda mandando mensagem para uma sessão par: uma resposta " +
		"enviada por qualquer outro caminho se perde. Mantenha a resposta um relatório conciso " +
		"(o que foi feito, onde e o que falta), nunca uma transcrição: quem enviou paga para ler " +
		"cada byte, e o detalhe já está nos seus commits e arquivos.",
	WorkerTask: "[lich] Tarefa da sessão %[1]q, que abriu esta sessão como seu subagente. " +
		"A sua mensagem final, quando terminar, volta para ela como o seu relatório.\n\n%[2]s",
	PickTicketNudge: "[lich] O seu turno terminou sem resposta enviada, então %[1]d pedidos voltaram " +
		"sem resposta para quem os enviou:\n%[2]s\nNada fora desta sessão sabe dizer qual deles era " +
		"aquele turno, e por isso nenhum pôde ser respondido por você. Um em que você ainda está " +
		"trabalhando ainda aceita a resposta, e toda resposta daqui em diante precisa nomear o " +
		"seu ticket: lich reply <ticket> \"<resposta>\".",
	NudgeResultsReady: Plural{
		One:   "A tarefa que você enviou para %[2]s tem o resultado pronto",
		Other: "Os resultados de %[1]d tarefas que você enviou estão prontos (%[2]s)",
	},
	NudgeRouteCLI:   "rode:\n  \"$LICH_BIN\" wait",
	NudgeRouteTools: "chame a ferramenta do lich `%s` sem ticket, ou rode:\n  \"$LICH_BIN\" wait",
	NudgeNotice:     "[lich] %[1]s. Para coletar tudo de uma vez, %[2]s",

	SubagentReport: "[lich] A sessão %[1]q%[2]s terminou a tarefa que você entregou a ela (ticket %[3]s). " +
		"O relatório dela:\n\n%[4]s",
	SubagentReportBranch: " na branch %s",
	BlockedNotice: "[lich] A sessão %q, o subagente que você abriu, está esperando uma confirmação de " +
		"permissão no card dela. A tarefa continua aberta: abra esse card para responder.",
	ReportSummary: Plural{
		One:   "a sessão do lich %s terminou",
		Other: "as sessões do lich %s terminaram",
	},
	BlockedSummary: "a sessão do lich %q está esperando uma confirmação de permissão",
	MergeNotice: "[lich] O pull request #%[1]d %[2]q (branch %[3]s) foi mergeado em %[4]s pelo lich. " +
		"Isto é um aviso, não uma tarefa: atualize o que você guarda sobre o estado deste trabalho e não faça mais nada.",
	MergeSummary: "Pull request #%[1]d mergeado em %[2]s",
	LateNotice:   "[lich] Agendado para %[1]s, entregue com %[2]s de atraso.\n\n",
	ResumePrompt: "[lich] O seu limite de uso foi renovado. Continue a tarefa em que você estava trabalhando.",

	CopyNotice: "[lich] cópia de %s; o original não é acessível a partir desta sessão, as edições ficam na cópia.",

	MCPInstructions: `O lich roda sessões de agentes de código lado a lado em uma janela; estas ferramentas são como esta sessão trabalha com as outras.

Delegue em paralelo: para o trabalho que pode rodar ao lado do seu, abra uma sessão de trabalho na própria git worktree (open_session com worktree) para ela ter o checkout dela e, depois, entregue a tarefa com send_to_session. Consulte list_worktrees antes: uma branch que já está em checkout é aberta, não criada. As sessões de trabalho são sessões completas na tela do usuário: visíveis, guiáveis, e suas para fechar (close_session) quando o trabalho termina. Essa é a diferença para os subagentes que o seu próprio harness roda, e é o que o usuário pede quando manda distribuir o trabalho entre branches ou worktrees: um subagente não tem checkout próprio nem card que o usuário possa abrir, ler ou assumir no meio da tarefa. Use um deles para ler algo e descartar, não para rodar a implementação que alguém pediu para ver. A exceção é uma sessão do Claude Code cujo lich-plugin transforma um subagente general-purpose em um desses cards: ali a sua ferramenta Agent abre a sessão de trabalho, em segundo plano, e o relatório volta sozinho, então distribua o trabalho com ela e abra sessões aqui para o que um subagente não pode ser.

Nunca faça polling por resultados. Um envio que passa do tempo de espera devolve um ticket e você segue com o seu próprio trabalho; quando os resultados estão prontos, uma nota curta [lich] chega ao seu prompt: colete tudo de uma vez com wait_for_answer (sem ticket). Essa nota só chega entre os seus turnos; então, enquanto um turno longo seu está rodando, olhe com wait_for_answer e no_wait nos pontos em que um resultado mudaria o que você faz (antes de delegar mais, depois de uma validação longa, antes da síntese final): ele devolve o que está pronto e quem ainda deve um resultado, sem esperar. Um olhar por ponto de decisão: se nada está pronto, siga em frente. Chamar de novo em loop é polling, e gasta os tokens que este desenho existe para poupar.

Um subagente ou etapa de workflow rodando dentro desta sessão não é o agente dela, e o lich não consegue distingui-los: ele envia e abre com private, e depois espera pelos próprios tickets. A nota [lich] e o wait_for_answer sem ticket pertencem ao agente da própria sessão.

Quando uma mensagem [lich] com ticket chega ao SEU prompt, você é a sessão de trabalho: faça a tarefa e responda com %s, um relatório conciso (o que foi feito, onde, o que falta), nunca uma transcrição.`,
	HandOverFailed: "A tarefa não chegou a ela: %[1]v. A sessão está aberta, então entregue a tarefa com " +
		"%[2]s depois de resolver o que isso diz.",
	SendToSessionPrivate: "send_to_session com private",
	OutcomeUnread: "A sessão %[1]q não pegou a tarefa: ela foi digitada naquele prompt e " +
		"nada a leu, então outra coisa está com aquele terminal: um provider ainda " +
		"iniciando, ou uma pergunta própria na tela (o Claude Code pergunta se um " +
		"diretório é confiável na primeira vez que roda nele). Nada ficou na fila e " +
		"nada foi respondido. Diga ao usuário para abrir o card %[1]q e resolver o que " +
		"está nele; depois disso a tarefa precisa ser enviada de novo.",
	OutcomeUndelivered: "A tarefa nunca chegou à sessão %[1]q: ela foi retida porque aquele terminal " +
		"não estava em um prompt (um script de setup da worktree ainda rodando, um provider que " +
		"nunca subiu) e a sessão terminou ou ficou assim por tempo demais. Nada " +
		"ficou na fila e nada foi respondido. Diga ao usuário para abrir o card %[1]q " +
		"para ver o que aconteceu ali; depois disso a tarefa precisa ser enviada de novo.",
	OutcomeUnanswered: "A sessão %[1]q terminou o turno dela sem responder pelo lich. " +
		"O que ela produziu está naquela sessão: diga ao usuário para abrir o card %[1]q para ler. " +
		"Se ela ainda está trabalhando em segundo plano, a resposta ainda pode chegar neste ticket " +
		"por uma hora.",
	OutcomeStopped: "A sessão %q foi fechada antes de responder, então a tarefa foi interrompida e nenhuma " +
		"resposta vai chegar.",
	OutcomeExpired: "A sessão %[1]q não respondeu dentro da hora em que uma tarefa fica aberta, então a tarefa " +
		"expirou e nenhuma resposta vai chegar. Abra o card %[1]q para ver em que pé ela está.",
	StillWorkingPrivate: "%[1]s ainda está trabalhando. O pedido é privado: nenhuma nota vai anunciar o resultado e " +
		"wait_for_answer sem ticket nunca o devolve. Chame wait_for_answer com " +
		"ticket %[2]q para segurar a linha por ele; esperar pelo seu próprio ticket não é polling.",
	StillWorkingOpen: "%[1]s ainda está trabalhando. O pedido está aberto: se aquela sessão ainda não estava em um prompt, " +
		"a tarefa fica retida e entra quando estiver. Quando o resultado estiver pronto, uma nota curta " +
		"vai chegar ao seu próprio prompt, então siga em frente, não há nada para consultar. Colete " +
		"com wait_for_answer (ticket %[2]q, ou sem ticket para tudo de uma vez).",
	StillWorkingPrivateCLI: "%[1]s ainda está trabalhando. O pedido é privado: nenhuma nota será digitada no prompt da " +
		"sessão que enviou, e `lich wait` sem ticket não o devolve. " +
		"Segure a linha por ele com:\n  lich wait %[2]s\n",
	StillWorkingOpenCLI: "%[1]s ainda está trabalhando. O pedido está aberto (uma mensagem para a qual aquela sessão não estava " +
		"pronta fica retida até ela estar) e uma nota será digitada no prompt da sessão que " +
		"enviou quando o resultado estiver pronto. Para segurar a linha por ele:\n" +
		"  lich wait %[2]s\n",
	CollectedNothing:      "Nada para coletar: nenhum resultado está esperando e nenhum pedido seu está aberto.",
	CollectedAnswer:       "Resposta de %[1]q (ticket %[2]s):\n%[3]s",
	CollectedStillWorking: "Ainda trabalhando: %s. Os resultados deles vão se anunciar no seu prompt.",
	OpenedProject:         "projeto %q",
	OpenedWorktree:        "%[1]s, na worktree %[2]s",
	OpenedConfined: " Ela roda confinada: um home vazio com só o estado do próprio agente, a " +
		"máquina somente leitura, e escrita só dentro do checkout dela.",
	OpenedSession: "Sessão %[1]q (%[2]s) aberta em %[3]s.%[4]s\n" +
		"Ela atende por %[1]q e por %[5]q. O agente dela pode ainda estar iniciando (uma " +
		"worktree nova roda antes o script de setup do projeto), então uma tarefa que você " +
		"enviar fica retida até o agente subir, em vez de se perder.\n",
	ClosedRemoved: "%[1]q fechada e a worktree %[2]s removida.\n",
	ClosedKept: "%[1]q fechada. A worktree %[2]s continua lá, e a sessão está estacionada: abrir " +
		"uma sessão nessa branch de novo retoma a conversa dela.\n",
	ClosedPlain: "%q fechada.\n",
}

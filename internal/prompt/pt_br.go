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
}

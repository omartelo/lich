package prompt

var es = Catalog{
	isOne: isOneSpanish,

	BriefingIntro: "Estás ejecutándote dentro de lich, que corre sesiones de agentes de código lado a lado y " +
		"puede abrir más junto a esta: cada una es una tarjeta que el usuario sigue y puede asumir " +
		"a mitad de la tarea, en su propia git worktree cuando el trabajo necesita su propio checkout. ",
	BriefingCards: "Aquí tu propia herramienta Agent las abre: un subagente general-purpose corre como " +
		"una de esas tarjetas, en este checkout salvo que pidas isolation 'worktree', en segundo " +
		"plano, y su informe te llega solo. Reparte el trabajo con ella. Abre tú mismo una " +
		"sesión solo para lo que un subagente no puede ser: otro tipo de agente, una rama que " +
		"el usuario nombró, o un trabajo que debe sobrevivir a esta sesión. ",
	BriefingNative: "Esta sesión es en sí misma una tarjeta de subagente que abrió otra sesión, y aquí tu " +
		"propia herramienta Agent corre un subagente dentro de esta sesión en lugar de como tarjeta. " +
		"Reparte el trabajo con ella y no con nuevas sesiones de lich: tu mensaje final es el " +
		"informe que espera la sesión que te abrió.",
	BriefingSessions: "Cuando haya que repartir el trabajo (varias tareas a la vez, una por rama o " +
		"checkout), esas sesiones son las que debes abrir, no los subagentes de tu propio " +
		"harness: un subagente no tiene checkout, ni tarjeta, ni nada que el usuario pueda " +
		"guiar o reanudar. ",
	BriefingCommandTools: "Las herramientas de lich de tu lista abren una y le entregan la tarea.",
	BriefingCommandCLI: "Abre una con `lich open --worktree BRANCH --prompt 'la tarea'`, que abre la " +
		"sesión y le entrega la tarea en un solo comando.",

	RelayMessage:  "[lich] %[1]s, no de tu propio prompt.\n\n%[2]s\n\n%[3]s",
	OriginCLI:     "Mensaje retransmitido por la línea de comandos de lich",
	OriginSession: "Mensaje de la sesión %q",
	ReplyCommand:  "  \"$LICH_BIN\" reply %s \"<tu respuesta>\"",
	ReplyRouteCLI: "Cuando tengas una respuesta, devuélvela ejecutando:\n%s",
	ReplyRouteTools: "Cuando tengas una respuesta, devuélvela con la herramienta de lich `%[1]s` " +
		"(ticket %[2]s), o ejecutando:\n%[3]s",
	ReplyOnlyWayBack: "Ese ticket es el único camino de vuelta: quien preguntó está bloqueado en él y " +
		"no lee nada más. No respondas enviando un mensaje a una sesión par: una respuesta " +
		"enviada por cualquier otro camino se pierde. Mantén la respuesta como un informe conciso " +
		"(qué se hizo, dónde y qué falta), nunca una transcripción: quien lo envió paga por leer " +
		"cada byte, y el detalle ya está en tus commits y archivos.",
	WorkerTask: "[lich] Tarea de la sesión %[1]q, que abrió esta sesión como su subagente. " +
		"Tu mensaje final, cuando termines, vuelve a ella como tu informe.\n\n%[2]s",
	PickTicketNudge: "[lich] Tu turno terminó sin enviar ninguna respuesta, así que %[1]d peticiones " +
		"volvieron sin respuesta a quienes las enviaron:\n%[2]s\nNada fuera de esta sesión puede " +
		"decir cuál de ellas era ese turno, y por eso ninguna pudo responderse por ti. Una en la " +
		"que sigues trabajando todavía acepta su respuesta, y toda respuesta a partir de ahora " +
		"tiene que nombrar su ticket: lich reply <ticket> \"<respuesta>\".",
	NudgeResultsReady: Plural{
		One:   "La tarea que enviaste a %[2]s tiene su resultado listo",
		Other: "Los resultados de %[1]d tareas que enviaste están listos (%[2]s)",
	},
	NudgeRouteCLI:   "ejecuta:\n  \"$LICH_BIN\" wait",
	NudgeRouteTools: "llama a la herramienta de lich `%s` sin ticket, o ejecuta:\n  \"$LICH_BIN\" wait",
	NudgeNotice:     "[lich] %[1]s. Para recogerlo todo de una vez, %[2]s",

	SubagentReport: "[lich] La sesión %[1]q%[2]s terminó la tarea que le entregaste (ticket %[3]s). " +
		"Su informe:\n\n%[4]s",
	SubagentReportBranch: " en la rama %s",
	BlockedNotice: "[lich] La sesión %q, el subagente que abriste, está esperando una confirmación de " +
		"permiso en su tarjeta. Su tarea sigue abierta: abre esa tarjeta para responder.",
	ReportSummary: Plural{
		One:   "la sesión de lich %s terminó",
		Other: "las sesiones de lich %s terminaron",
	},
	BlockedSummary: "la sesión de lich %q está esperando una confirmación de permiso",
	MergeNotice: "[lich] El pull request #%[1]d %[2]q (rama %[3]s) se fusionó en %[4]s desde lich. " +
		"Esto es un aviso, no una tarea: actualiza lo que guardes sobre el estado de este trabajo y no hagas nada más.",
	MergeSummary: "Pull request #%[1]d fusionado en %[2]s",
	LateNotice:   "[lich] Programado para %[1]s, entregado con %[2]s de retraso.\n\n",
	ResumePrompt: "[lich] Tu límite de uso se restableció. Continúa la tarea en la que estabas trabajando.",

	CopyNotice: "[lich] copia de %s; el original no es accesible desde esta sesión, las ediciones se quedan en la copia.",

	MCPInstructions: `lich corre sesiones de agentes de código lado a lado en una ventana; estas herramientas son la forma en que esta sesión trabaja con las demás.

Delega en paralelo: para el trabajo que puede correr junto al tuyo, abre una sesión de trabajo en su propia git worktree (open_session con worktree) para que tenga su propio checkout y luego entrégale la tarea con send_to_session. Consulta list_worktrees antes: una rama que ya está en checkout se abre, no se crea. Las sesiones de trabajo son sesiones completas en la pantalla del usuario: visibles, guiables y tuyas para cerrar (close_session) cuando el trabajo termina. Esa es la diferencia con los subagentes que corre tu propio harness, y es lo que el usuario pide cuando dice que repartas el trabajo entre ramas o worktrees: un subagente no tiene checkout propio ni una tarjeta que el usuario pueda abrir, leer o asumir a mitad de la tarea. Usa uno de ellos para leer algo y descartarlo, no para correr la implementación que alguien pidió ver. La excepción es una sesión de Claude Code cuyo lich-plugin convierte un subagente general-purpose en una de esas tarjetas: ahí tu herramienta Agent abre la sesión de trabajo, en segundo plano, y su informe te llega solo, así que reparte el trabajo con ella y abre sesiones aquí para lo que un subagente no puede ser.

Nunca hagas polling de resultados. Un envío que supera su espera devuelve un ticket y sigues con tu propio trabajo; cuando los resultados están listos, una nota corta [lich] llega a tu prompt: recógelo todo de una vez con wait_for_answer (sin ticket). Esa nota solo llega entre tus turnos; por eso, mientras corre un turno largo tuyo, mira con wait_for_answer y no_wait en los puntos en que un resultado cambiaría lo que haces (antes de delegar más, después de una validación larga, antes de la síntesis final): devuelve lo que está listo y quién aún debe uno, sin esperar. Una mirada por punto de decisión: si no hay nada listo, sigue adelante. Volver a llamarlo en bucle es polling, y gasta los tokens que este diseño existe para ahorrar.

Un subagente o paso de workflow que corre dentro de esta sesión no es su agente, y lich no puede distinguirlos: envía y abre con private, y luego espera sus propios tickets. La nota [lich] y el wait_for_answer sin ticket pertenecen al agente de la propia sesión.

Cuando un mensaje [lich] con ticket llega a TU prompt, eres la sesión de trabajo: haz la tarea y responde con %s, un informe conciso (qué se hizo, dónde, qué falta), nunca una transcripción.`,
	HandOverFailed: "La tarea no le llegó: %[1]v. La sesión está abierta, así que entrégale la tarea con " +
		"%[2]s cuando resuelvas lo que eso dice.",
	SendToSessionPrivate: "send_to_session con private",
	OutcomeUnread: "La sesión %[1]q no tomó la tarea: se escribió en ese prompt y " +
		"nada la leyó, así que otra cosa tiene esa terminal: un proveedor que aún " +
		"está iniciando, o una pregunta propia en pantalla (Claude Code pregunta si un " +
		"directorio es de confianza la primera vez que corre en él). No quedó nada en cola y " +
		"no se respondió nada. Dile al usuario que abra la tarjeta %[1]q y resuelva lo que " +
		"hay en ella; después de eso hay que volver a enviar la tarea.",
	OutcomeUndelivered: "La tarea nunca llegó a la sesión %[1]q: se retuvo porque esa terminal " +
		"no estaba en un prompt (un script de preparación de la worktree aún corriendo, un " +
		"proveedor que nunca arrancó) y la sesión terminó o se quedó así demasiado tiempo. Ya no " +
		"queda nada en cola y no se respondió nada. Dile al usuario que abra la tarjeta %[1]q " +
		"para ver qué pasó ahí; después de eso hay que volver a enviar la tarea.",
	OutcomeUnanswered: "La sesión %[1]q terminó su turno sin responder a través de lich. " +
		"Lo que produjo está en esa sesión: dile al usuario que abra la tarjeta %[1]q para leerlo. " +
		"Si aún sigue trabajando en segundo plano, su respuesta todavía puede llegar a este ticket " +
		"durante una hora.",
	OutcomeStopped: "La sesión %q se cerró antes de responder, así que la tarea se detuvo y no " +
		"llegará ninguna respuesta.",
	OutcomeExpired: "La sesión %[1]q no respondió dentro de la hora que una tarea se mantiene abierta, así que la " +
		"tarea expiró y no llegará ninguna respuesta. Abre la tarjeta %[1]q para ver cómo va.",
	StillWorkingPrivate: "%[1]s sigue trabajando. El encargo es privado: ninguna nota anunciará su resultado y " +
		"wait_for_answer sin ticket nunca lo devuelve. Llama a wait_for_answer con " +
		"el ticket %[2]q para mantener la línea por él: esperar tu propio ticket no es polling.",
	StillWorkingOpen: "%[1]s sigue trabajando. El encargo está abierto: si esa sesión aún no estaba en un prompt, " +
		"la tarea queda retenida y entra cuando lo esté. Cuando su resultado esté listo, una nota corta " +
		"llegará a tu propio prompt, así que sigue adelante: no hay nada que consultar. Recógelo " +
		"con wait_for_answer (ticket %[2]q, o sin ticket para todo de una vez).",
	StillWorkingPrivateCLI: "%[1]s sigue trabajando. El encargo es privado: no se escribirá ninguna nota en el prompt de la " +
		"sesión que lo envió, y `lich wait` sin ticket no lo devuelve. " +
		"Mantén la línea por él con:\n  lich wait %[2]s\n",
	StillWorkingOpenCLI: "%[1]s sigue trabajando. El encargo está abierto (un mensaje para el que esa sesión no estaba " +
		"lista queda retenido hasta que lo esté) y se escribirá una nota en el prompt de la sesión que " +
		"lo envió cuando su resultado esté listo. Para mantener la línea por él en su lugar:\n" +
		"  lich wait %[2]s\n",
	CollectedNothing:      "Nada que recoger: no hay resultados esperando y ningún encargo tuyo está abierto.",
	CollectedAnswer:       "Respuesta de %[1]q (ticket %[2]s):\n%[3]s",
	CollectedStillWorking: "Siguen trabajando: %s. Sus resultados se anunciarán solos en tu prompt.",
	OpenedProject:         "proyecto %q",
	OpenedWorktree:        "%[1]s, en la worktree %[2]s",
	OpenedConfined: " Corre confinada: un home vacío que solo contiene el estado propio de su agente, la " +
		"máquina en solo lectura y escritura únicamente dentro de su checkout.",
	OpenedSession: "Sesión %[1]q (%[2]s) abierta en %[3]s.%[4]s\n" +
		"Responde por %[1]q y por %[5]q. Su agente puede estar aún iniciando (una worktree " +
		"nueva ejecuta primero el script de preparación del proyecto), así que una tarea que le " +
		"envíes queda retenida hasta que el agente esté activo en lugar de perderse.\n",
	ClosedRemoved: "Se cerró %[1]q y se eliminó su worktree %[2]s.\n",
	ClosedKept: "Se cerró %[1]q. Su worktree %[2]s sigue ahí y la sesión queda aparcada: volver a abrir " +
		"una sesión en esa rama retoma su conversación.\n",
	ClosedPlain: "Se cerró %q.\n",
}

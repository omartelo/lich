import type { Shape } from "../../catalog"
import type { shell as en } from "../en/shell"

export const shell = {
  agentPluginGate: {
    updateAvailable: "El plugin de lich {version} está disponible",
    update: "Actualizar",
    later: "Más tarde",
    installVersion: "Instalar la v{version}",
    updating: "Actualizando el plugin de lich…",
    updated: "Plugin actualizado. Reinicia tus sesiones para aplicarlo.",
    updateFailed: "Falló la actualización",
    installed: "Plugin instalado. Reinicia tus sesiones para aplicarlo.",
    codexTrust:
      "Codex: ejecuta /hooks en una sesión de Codex para confiar en los hooks del plugin.",
    installFailed: "Falló la instalación: {error}",
    title: "Activa la integración con agentes",
    description:
      "El plugin de lich informa a la aplicación del estado y los títulos de tus sesiones, actualiza su estado de git y permite que una sesión restaurada reanude su conversación. Instálalo en las CLI que usas.",
    needsHooks: "necesita /hooks para confiar en él",
    dontAskAgain: "No volver a preguntar",
    notNow: "Ahora no",
    installing: "Instalando…",
    install: "Instalar",
  },
  appUpdateGate: {
    available: "lich {version} está disponible",
    updateAndInstall: "Actualizar e instalar",
    later: "Más tarde",
    updated: "lich se actualizó a la {version}",
    retry: "Reintentar",
    restart: "Reiniciar",
    restartFailed: "No se pudo reiniciar: {error}",
    install: "Instalar",
    viewRelease: "Ver la versión",
  },
  confirmDialog: {
    cancel: "Cancelar",
  },
  emptySessions: {
    title: "No hay ninguna sesión abierta",
    description: "Abre una sesión para empezar a trabajar en este proyecto.",
    noAgent:
      "No se encontró ningún agente en el PATH, así que esto abre una terminal. Configura uno en Ajustes › Proveedores.",
    newTerminal: "Nueva terminal",
    newSession: "Nueva sesión",
  },
  fileTree: {
    openInEditor: "Abrir en el editor",
    expandAll: "Expandir todo",
    collapseAll: "Contraer todo",
  },
  footerBar: {
    attachFailed: "No se pudo adjuntar el archivo",
    attachFile: "Adjuntar archivo",
    browseCode: "Explorar el código",
    reviewChanges: "Revisar cambios",
    viewPullRequest: "Ver pull request",
    pullRequest: "PR #{number}",
    leftFooter: "Pie de ventana izquierdo",
    rightFooter: "Pie de ventana derecho",
  },
  footerSession: {
    contextWindow: "Ventana de contexto",
    contextTokens: "{tokens} / {window} tokens",
    costUnavailable: "Costo no disponible",
    sessionCost: "Costo de la sesión",
    costOfBudget: "{cost} de {budget} de presupuesto",
    costNote:
      "Costo de API de las conversaciones de esta sesión. Los cargos de la suscripción pueden diferir.",
    handsOnTime: "Tiempo activo",
    handsOnFailed: "No se pudo leer el tiempo activo.",
  },
  gitMissingGate: {
    title: "git no está instalado",
    description:
      "lich lee cada rama, diff y worktree a través de git. Las sesiones siguen funcionando sin él, pero las superficies de control de versiones quedan vacías.",
    notNow: "Ahora no",
    installGit: "Instalar git",
  },
  home: {
    title: "No hay ningún proyecto abierto",
    description: "Abre una carpeta para iniciar una sesión de terminal.",
    openProject: "Abrir proyecto",
  },
  notificationsOptIn: {
    title: "¿Avisarte cuando una sesión te necesite?",
    description:
      "Una sesión acaba de pedirte una respuesta mientras estabas en otro sitio. lich puede mostrar una notificación de escritorio cuando eso pase, con el nombre de la sesión y de su proyecto, para que puedas iniciar algo y alejarte de la ventana. Las sesiones que estás mirando nunca notifican. Puedes cambiarlo cuando quieras en Ajustes › Notificaciones.",
    noThanks: "No, gracias",
    notifyMe: "Avisarme",
  },
  paneSeams: {
    resizeColumns: "Cambiar el tamaño de las columnas",
    resizeRows: "Cambiar el tamaño de las filas",
  },
  patchNotesDialog: {
    title: "Novedades de lich {version}",
    viewChangelog: "Ver el changelog completo",
    gotIt: "Entendido",
  },
  planQuota: {
    planUsage: "Uso del plan",
    locked: "Bloqueado",
  },
  providerSetupDialog: {
    chooseTitle: "Elige tus agentes",
    noneTitle: "No se encontraron agentes de código",
    chooseDescription:
      "lich encontró estos en tu máquina. Activa los que uses: el resto queda fuera del menú Nueva sesión.",
    noneDescription:
      "lich ejecuta los agentes que ya tienes instalados en tu máquina y no encontró ninguno. Instala uno y vuelve a comprobar.",
    changeLater: "Puedes cambiarlo cuando quieras en Ajustes › Proveedores.",
    pointToBinary: "¿Ya tienes uno instalado? Indica a lich su binario en Ajustes › Proveedores.",
    continue: "Continuar",
  },
  quotaGauge: {
    aheadOfPace: "Por delante del ritmo",
    aheadTooltip: "Gastando por delante del ritmo de esta ventana.",
    locked: "Bloqueado",
  },
  resumeSessionDialog: {
    title: "¿Reanudar la sesión anterior?",
    description:
      "{label} dejó una conversación ({id}). Reanúdala para retomarla donde se detuvo, o empieza de cero para una vacía.",
    alwaysFor: "Hacer siempre esto para {provider}",
    changeInSettings: "· cámbialo en Ajustes › Proveedores",
    thisProvider: "este proveedor",
    startNew: "Empezar de cero",
    resume: "Reanudar",
  },
  shortcutsOverlay: {
    title: "Atajos de teclado",
    rebind: "Reasígnalos en Ajustes › Atajos",
    close: "cerrar",
  },
  uncleanExitGate: {
    title: "La ejecución anterior de lich terminó de forma inesperada",
    description: "Tus sesiones se restauraron, pero lo que estaban ejecutando se detuvo con ella.",
    openLogFolder: "Abrir carpeta del log",
    opening: "Abriendo la carpeta del log…",
    opened: "Carpeta del log abierta.",
    openFailed: "No se pudo abrir la carpeta del log",
    dismiss: "Descartar",
  },
  updateProgressToast: {
    downloading: "Descargando lich {version}",
    installing: "Instalando lich {version}…",
    reopens: "lich se cierra y se vuelve a abrir solo.",
  },
  app: {
    stage: "El escenario",
    reloadStage: "Recargar el escenario",
    screen: "Esta pantalla",
    panel: "El panel",
  },
  projects: {
    relocated: "{name} ahora abre {path}",
    relocateFailed: "No se pudo reubicar: {error}",
    noLongerAvailable: "{label} ya no está disponible para reanudar",
    bringBackFailed: "No se pudo recuperar {label}",
    closed: "Se cerró {label}",
    undo: "Deshacer",
    entrypointSet: "Punto de entrada definido",
    entrypointSetDescription: "Se ejecuta la próxima vez que se inicie esta terminal.",
    entrypointCleared: "Punto de entrada borrado",
    entrypointClearedDescription: "Esta terminal vuelve a iniciar un shell normal.",
    entrypointFailed: "No se pudo guardar el punto de entrada: {error}",
    scheduled: "Programado {when}",
    scheduledNow: "Programado para ahora",
    scheduleCleared: "Programación borrada",
    scheduleFailed: "No se pudo programar: {error}",
  },
  projectEvents: {
    unlabeledSession: "Una sesión",
    scheduleLost: "Prompt programado perdido junto con {label}",
    scheduleDue: "Vence {when}: {prompt}",
    relayAnswered: "{target} respondió en su propia sesión",
    relayDetail: "Terminó sin responder a través de lich: ábrela para leer la respuesta.",
    open: "Abrir",
    needsInput: "{label} necesita una respuesta tuya",
    finished: "{label} terminó de trabajar",
  },
} satisfies Shape<typeof en>

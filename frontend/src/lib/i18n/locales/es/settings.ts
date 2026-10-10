import type { Shape } from "../../catalog"
import type { settings as en } from "../en/settings"

export const settings = {
  language: {
    uiTitle: "Idioma de la interfaz",
    uiDescription: "El idioma de las pantallas propias de lich.",
    promptTitle: "Idioma de los prompts",
    promptDescription:
      "El idioma del texto que lich entrega a tus agentes: briefings, mensajes retransmitidos y avisos. Las sesiones que se inicien desde ahora lo usan en todo; las abiertas lo reciben solo en los mensajes retransmitidos.",
    uiSearchWords:
      "idioma language locale traducción inglés english portugués portuguese español spanish",
    promptSearchWords:
      "idioma language locale traducción agente briefing relay inglés english portugués portuguese español spanish",
    promptSaveFailed: "No se pudo guardar el idioma de los prompts: {error}",
  },
  settings: {
    search: "Buscar ajustes",
    resultCount: { one: "{count} ajuste", other: "{count} ajustes" },
    noMatch: "Nada coincide con {query}. La búsqueda lee el nombre de cada ajuste, no sus valores.",
    section: {
      appearance: "Apariencia",
      notifications: "Notificaciones",
      hotkeys: "Atajos",
      providers: "Proveedores",
      sandbox: "Sandbox",
      versionControl: "Control de versiones",
      updates: "Actualizaciones",
      help: "Ayuda",
    },
  },
  appearanceSettings: {
    importPickTitle: "Importar tema",
    templatePickTitle: "Guardar plantilla de tema",
    imported: "Tema importado: {name}",
    importFailed: "No se pudo importar el tema: {error}",
    installed: {
      one: "Se instaló {count} tema de {pack} v{version}",
      other: "Se instalaron {count} temas de {pack} v{version}",
    },
    installFailed: "No se pudo instalar el tema: {error}",
    upToDate: "{pack} ya está en la v{version}",
    updated: "{pack} se actualizó a la v{version}",
    updateFailed: "No se pudo actualizar el tema: {error}",
    removed: "Tema eliminado: {name}",
    removeFailed: "No se pudo eliminar el tema: {error}",
    templateSaved: "Plantilla de tema guardada en {path}",
    templateFailed: "Falló la plantilla de tema: {error}",
    zoomTitle: "Zoom",
    zoomOut: "Alejar",
    zoomIn: "Acercar",
    resetZoom: "Restablecer el zoom",
    resetTextSize: "Restablecer el tamaño del texto de la terminal",
    textSizeTitle: "Tamaño del texto de la terminal",
    textSmaller: "Texto de terminal más pequeño",
    textLarger: "Texto de terminal más grande",
    replaceThemesTitle: "¿Reemplazar los temas importados?",
    replaceThemesDescription:
      "El repositorio contiene temas que ya están instalados: {themes}. ¿Instalar de todos modos? Esto elimina de forma permanente las versiones anteriores.",
    replaceThemes: "Reemplazar temas",
    removeThemeTitle: "¿Eliminar el tema importado?",
    removeThemeDescription: "¿Eliminar {name}? Esto quita el archivo del tema importado de lich.",
    removeTheme: "Eliminar tema",
    replaceThemeTitle: "¿Reemplazar el tema importado?",
    replaceThemeDescription:
      "Ya existe un tema con el id {id}. ¿Importar de todos modos? Esto elimina de forma permanente el tema anterior.",
    replaceTheme: "Reemplazar tema",
  },
  themePicker: {
    title: "Tema",
    searchWords: "colores oscuro claro terminal paleta colors dark light palette",
    description: "Da color a la interfaz y a la terminal.",
    choose: "Elige un tema",
    system: "Sistema",
    followsOs: "sigue al sistema operativo",
    inUse: "en uso",
    bundled: "incluido",
    count: { one: "{count} tema", other: "{count} temas" },
    import: "Importar",
    updateNamed: "Actualizar {name}",
    updateHint: "Busca una versión más reciente en el repositorio",
    removeNamed: "Eliminar {name}",
  },
  brokenThemeList: {
    label: "Temas que no se pueden cargar",
    heading: { one: "{count} tema no se puede cargar", other: "{count} temas no se pueden cargar" },
    removeNamed: "Eliminar {name}",
    remove: "Eliminar",
  },
  importThemeDialog: {
    title: "Importar tema",
    description:
      "Instala desde un repositorio git para mantener el tema versionado, o elige un único archivo de tema.",
    repositoryUrl: "URL del repositorio",
    install: "Instalar",
    installing: "Instalando…",
    repositoryHint:
      "Un repositorio que contiene {file} y uno o más archivos de tema. La versión de su manifiesto es lo que lich compara al actualizar.",
    or: "o",
    chooseFile: "Elegir archivo…",
    fileHint: "Un único archivo de tema. Sin versión ni actualizaciones.",
    downloadTemplate: "Descargar plantilla",
  },
  fontSetting: {
    title: "Fuente de la terminal",
    searchWords: "tipografía monoespaciada typeface monospace font",
    sample: "el veloz murciélago hindú 0O1lI",
    search: "Buscar fuentes",
    empty: "Ninguna fuente coincide.",
  },
  footerSettings: {
    title: "Pie de ventana",
    searchWords: "barra de estado diseño ordenar status bar layout arrange footer",
    loading: "Cargando las opciones guardadas…",
    saving: "Guardando…",
    description:
      "Arrastra un elemento entre los lados, o fuera para ocultarlo. Se aplica a todos los proyectos.",
    restoreDefault: "Restaurar valores predeterminados",
    saveFailed: "No se pudo guardar el pie de ventana: {error}",
    ceilingTitle: "Tope de gasto",
    ceilingSearchWords: "costo presupuesto usd cost budget",
    ceilingDescription:
      "Avisa cuando el costo de API de la sesión se acerca a este monto: ámbar al 80%, rojo al 95%. Vacío para ninguno.",
    ceilingPlaceholder: "Sin tope",
    ceilingLabel: "Tope de gasto de la sesión en dólares",
  },
  footerLayoutEditor: {
    zone: { available: "Disponibles", left: "Izquierda", right: "Derecha" },
    allPlaced: "Todos los elementos están en el pie de ventana",
    dragHere: "Arrastra elementos aquí",
    moveNamed: "Mover {name}",
    previewLabel: "Vista previa del pie de ventana",
    previewHint: "Vista previa · valores de ejemplo · desplázate para ver todos los elementos",
    previewScrollLabel: "Vista previa desplazable del pie de ventana",
    previewEmpty: "No hay elementos del pie de ventana seleccionados",
  },
  diffLayoutSetting: {
    title: "Diseño del diff",
    searchWords: "lado a lado dividido unificado revisión side by side split unified review",
    description:
      "Cómo se dibujan los cambios en la pestaña Revisión y en los pull requests. La vista en paralelo muestra la unificada mientras el panel es demasiado estrecho para dos columnas.",
    layout: { unified: "Unificado", split: "En paralelo" },
  },
  notificationsSettings: {
    needsInputTitle: "Avisarme cuando una sesión necesite una respuesta",
    needsInputDescription:
      "Muestra una notificación de escritorio cuando una sesión está bloqueada esperándote (una petición de permiso, una pregunta) y la ventana de lich no es en la que estás. Mientras estás en lich nada cambia: la campana de la tarjeta y el aviso emergente se encargan de avisar.",
    needsInputLabel:
      "Mostrar una notificación de escritorio cuando una sesión necesite una respuesta",
    finishedTitle: "Avisarme cuando una sesión termine de trabajar",
    finishedDescription:
      "Muestra una notificación de escritorio cuando una sesión termina su turno sin nada más que preguntarte y la ventana de lich no es en la que estás. Es independiente del ajuste anterior: una ejecución larga que dejaste atrás merece saberse aunque prefieras no enterarte de cada petición. No viene con aviso emergente: dentro de lich, la tarjeta y la pestaña de su proyecto ya lo indican.",
    finishedLabel: "Mostrar una notificación de escritorio cuando una sesión termine de trabajar",
  },
  hotkeysSettings: {
    alsoBound: "También asignado a {actions}",
    pressKeys: "Pulsa las teclas…",
    unassignNamed: "Quitar el atajo de {name}",
    unassignHint: "Deja esta acción sin atajo",
    resetNamed: "Restablecer el atajo de {name}",
    resetHint: "Restaura el atajo predeterminado",
    terminalNote:
      "Propio de lich y fijo: este tapa un acelerador del navegador, que responde a una combinación y no a aquello a lo que se reasigne.",
    passthroughNote:
      "lich los reescribe de camino a la terminal del agente, así que son fijos y no se pueden reasignar.",
  },
  providersPane: {
    detecting: "Detectando proveedores…",
    heading: "Proveedores",
    addProvider: "Añadir proveedor",
    moreDetected: "{count} más detectados",
    defaultTitle: "Proveedor predeterminado",
    defaultSearchWords: "agente nueva sesión agent new session",
    defaultDescription:
      "Qué proveedor lanzan las acciones de sesión implícitas: el atajo de nueva sesión, el botón Nueva sesión de un proyecto vacío y las worktrees recién creadas.",
    allProjects: "Todos los proyectos",
    defaultForAll: "Proveedor predeterminado de todos los proyectos",
    defaultForProject: "Proveedor predeterminado de {name}",
    clear: "Borrar",
    clearFallsBack: "Al borrarlo se usa {name}",
    following: "Siguiendo a todos los proyectos: {name}",
    openSettings: "Abrir los ajustes de {name}",
    defaultHere: "predeterminado aquí",
    customPath: "ruta personalizada",
    skipWorktrees: "worktrees",
    skipEverywhere: "sin preguntas",
    enableNamed: "Activar {name}",
  },
  providersSettings: {
    defaultDescription:
      "El proveedor que usan las acciones de sesión implícitas, salvo que el proyecto en el que estás elija el suyo en Ajustes › Proveedores.",
    defaultLabel: "Proveedor predeterminado global",
    detected: "Detectado en el PATH",
  },
  providerDetail: {
    back: "Todos los proveedores",
    enabled: "Activado",
    rightNowTitle: "Ahora mismo",
    rightNowSearchWords: "sesiones abiertas documentación open sessions docs",
    openSessions: { one: "{count} sesión abierta", other: "{count} sesiones abiertas" },
    documentation: "Documentación",
  },
  providerToggleRow: {
    notFound: "No se encontró en el PATH",
  },
  providerDocsLink: {
    howToInstall: "Cómo instalarlo",
  },
  providerBinary: {
    title: "Binario",
    searchWords: "ruta ejecutable comando path executable command binary",
    description: "Qué ejecutable lanza una sesión.",
    descriptionProject: "Qué ejecutable lanza una sesión en {name}.",
    pickTitle: "Elige el binario de {provider}",
    pickerFailed: "No se pudo abrir el selector de archivos",
    projectOnly: "Solo {name}",
    allProjects: "Todos los proyectos",
    layersHint: "De dónde viene: gana la primera capa activada",
    done: "Listo",
    useLayer: "Usar el binario de {provider} definido para {layer}",
    layerPath: "Binario de {provider} para {layer}",
    pickLayer: "Elige el binario de {provider} para {layer}",
    browse: "Examinar",
    useDifferent: "Usar un binario distinto",
    sourceProjectNamed: "definido para {name}",
    sourceProject: "definido para este proyecto",
    sourceGlobal: "definido para todos los proyectos",
    sourcePath: "desde $PATH",
  },
  providerBinSettings: {
    skipTitle: "Omitir las peticiones de permiso",
    skipSearchWords: "yolo peligroso aprobaciones dangerous approvals skip permission",
    skipDescription:
      "Hasta dónde corre {provider} sin preguntar: edita archivos, ejecuta comandos e instala cosas sin confirmar, y lich lo lanza con {flag}.",
    skipLabel: "Hasta dónde corre {provider} sin preguntar",
    skip: {
      never: {
        label: "Nunca",
        consequence: "Cada edición y cada comando esperan por ti, en cada checkout.",
      },
      worktrees: {
        label: "Solo worktrees",
        consequence: "Las sesiones en el directorio del proyecto siguen preguntando.",
      },
      everywhere: {
        label: "En todas partes",
        consequence: "Incluido el árbol en el que trabajas. Nada preguntará.",
      },
    },
    confirmTitle: "Omitir las peticiones de permiso: {level}",
    confirmDescription:
      "{provider} editará archivos, ejecutará comandos e instalará cosas sin confirmar, lanzado con {flag}. {consequence}",
    confirm: "Omitir peticiones",
  },
  planUsageSetting: {
    title: "Uso del plan",
    searchWords: "cuota suscripción límite quota subscription limit",
    description:
      "Cuánto ha gastado tu suscripción de cada ventana, leído de tu cuenta y actualizado cada 5 minutos mientras lich está abierto.",
    signedOut: "Sin sesión iniciada.",
    signedOutRun: "Sin sesión iniciada. Ejecuta {command} para leer el uso del plan.",
    readFailed:
      "No se pudo leer el uso del plan. El endpoint de uso del proveedor no está documentado y puede haber cambiado.",
    used: "usado",
    resets: "se restablece",
  },
  closeSetting: {
    title: "Al cerrar la ventana",
    searchWords: "salir cerrar segundo plano ventana bandeja mantener",
    description:
      "Qué hace lich con sus sesiones cuando cierras su ventana. Sin ninguna en ejecución, cerrar la ventana sale de lich.",
    choice: {
      ask: { label: "Preguntar", consequence: "lich pregunta cada vez si sigue en segundo plano." },
      background: {
        label: "Mantener en ejecución",
        consequence:
          "La ventana se cierra y las sesiones siguen trabajando; abrir lich de nuevo las devuelve.",
      },
      quit: {
        label: "Salir",
        consequence: "Cerrar la ventana sale de lich y termina todas sus sesiones.",
      },
    },
  },
  restoreSetting: {
    title: "Sesiones restauradas",
    searchWords:
      "reanudar conversación nueva reiniciar preguntar resume conversation start new restart ask",
    description:
      "Qué hace por primera vez al abrirla una tarjeta que ejecutaba {provider} antes de que lich se reiniciara.",
    label: "Qué hace una tarjeta restaurada de {provider}",
    choice: {
      ask: {
        label: "Preguntar",
        consequence: "Cada tarjeta restaurada pregunta antes de abrirse.",
      },
      resume: {
        label: "Reanudar",
        consequence:
          "Las tarjetas restauradas continúan su conversación al abrirse. Para una vacía, abre una sesión nueva.",
      },
      fresh: {
        label: "Empezar de cero",
        consequence:
          "Las tarjetas restauradas se abren vacías. La conversación anterior no se retoma.",
      },
    },
  },
  pasteUnfoldSetting: {
    title: "Desplegar pegados largos",
    searchWords:
      "pegar pegado plegar chip resumen marcador texto paste fold chip summary placeholder",
    descriptionNextPaste:
      "Un pegado que {provider} plegaría en un marcador llega como el texto completo, para que puedas editarlo antes de enviarlo. Se aplica desde el próximo pegado, en todas las sesiones de {provider}.",
    descriptionNextSession:
      "Un pegado que {provider} plegaría en un marcador llega como el texto completo, para que puedas editarlo antes de enviarlo. Se aplica a las sesiones abiertas después de cambiarlo.",
    label: "Desplegar pegados largos en {provider}",
  },
  subagentCardsSetting: {
    title: "Subagentes como sesiones de lich",
    searchWords:
      "agente subagente tarjeta worktree delegar segundo plano agent subagent card delegate background",
    description:
      "Un subagente general-purpose abre su propia tarjeta y worktree en lugar de ejecutarse oculto dentro de {provider}. Se aplica a las sesiones abiertas después de cambiarlo.",
    label: "Ejecutar los subagentes de {provider} como sesiones de lich",
  },
  ultracodeSetting: {
    title: "Ultracode",
    searchWords: "workflow orquestación multiagente esfuerzo orchestration multi-agent effort",
    description:
      "Cada sesión de {provider} arranca con ultracode activado, orquestando flujos de trabajo multiagente con el esfuerzo con el que corra. Sigue activado cuando una sesión se reinicia o se reanuda.",
    label: "Iniciar las sesiones de {provider} en ultracode",
  },
  sandboxSettings: {
    rung: {
      off: "Desactivado",
      ask: "Preguntar",
      worktrees: "Worktrees",
      everywhere: "En todas partes",
    },
    exposes: {
      off: "Cada sesión corre en la máquina.",
      ask: "Una sesión cuya casilla desmarcas corre en la máquina.",
      worktrees: "Las sesiones en el directorio del proyecto corren en la máquina.",
    },
    summary:
      "Un home vacío, la máquina en solo lectura, escritura solo en el checkout. La red sigue activa.",
    confinedTitle: "Qué sesiones corren confinadas",
    confinedSearchWords: "aislar bwrap seatbelt isolate",
    confinedHint:
      "Preguntar lleva la pregunta al menú Nueva sesión y al diálogo Nueva worktree. Una sesión abierta donde nadie puede responder (por otra sesión o mediante la herramienta MCP) queda confinada. Las terminales nunca se confinan; el sandbox es para agentes que trabajan sin supervisión.",
    carryTitle: "Qué puede llevarse dentro una sesión confinada",
    sshTitle: "Agente SSH",
    sshSearchWords: "push claves keys",
    sshDescription:
      "git push funciona dentro. Firma con cada identidad de tu agente, contra cualquier host, mientras la sesión esté en ejecución.",
    sshLoaded: "Cargadas: {keys}",
    sshNone: "No hay nada cargado.",
    ghTitle: "Token de GitHub",
    ghSearchWords: "gh credenciales credentials",
    ghDescription:
      "gh funciona dentro. El agente puede leer el token de su entorno y gastarlo fuera de este repositorio.",
    ghAccount: "Como {login}.",
    ghActiveAccount: "Como la cuenta activa de gh.",
    rungLabel: "Qué sesiones de {provider} corren confinadas",
    confirmTitle: "Confinar menos sesiones de {provider}",
    confirmDescription: "{rung}: {exposes}",
    confirm: "Dejar sin confinar",
    canConfine: "Esta máquina puede confinar sesiones",
    cannotConfine: "Esta máquina no puede confinar sesiones",
    statusDetail: "· {detail}",
  },
  vcsToolsSetting: {
    title: "Herramientas de línea de comandos",
    searchWords: "git gh instalar install",
    description: "lich maneja git y la CLI de GitHub. Todo lo de esta pantalla pasa por ellas.",
    notOnPath: "No está en $PATH",
    install: "Instalar",
  },
  versionControlSettings: {
    account: "Cuenta de GitHub",
    accountFor: "Cuenta de GitHub para {name}",
    accountSearchWords: "login gh cambiar switch usuario",
    openProject: "Abre un proyecto para configurar su control de versiones.",
    accountDescription:
      "Con qué cuenta ejecuta lich gh para este proyecto: pull requests, comprobaciones y checkouts de PR. gh mantiene una cuenta activa por host, así que un repositorio que solo otra cuenta puede ver aparece como no encontrado. Las cuentas de un host empresarial se listan con él.",
    activeAccount: "la cuenta activa de gh",
    selectAccount: "Elige una cuenta",
    listFailed: "No se pudieron listar las cuentas de GitHub: {error}",
    noAccounts:
      "gh no ha iniciado sesión en ninguna cuenta. Ejecuta `gh auth login` en una terminal.",
  },
  updatesSettings: {
    applicationTitle: "Aplicación",
    applicationSearchWords: "versión actualizar version upgrade",
    applicationDescription: "lich {version}: busca actualizaciones al iniciar y cada hora.",
    checkForUpdates: "Buscar actualizaciones",
    available: "lich {version} está disponible: sigue las indicaciones.",
    upToDate: "Tienes la última versión.",
    checkFailed: "Falló la comprobación: ¿tienes conexión?",
    whatsNewTitle: "Novedades",
    whatsNewSearchWords: "changelog notas de la versión release notes",
    patchNotesFor: "Notas de la versión v{version}.",
    noPatchNotes: "No hay notas de la versión para esta compilación.",
    viewPatchNotes: "Ver las notas de la versión",
  },
  pluginSetting: {
    title: "Plugin de lich",
    searchWords: "hooks instalar install",
    description:
      "Estado de la sesión, títulos, actualización de git y reanudación, dentro de las CLI de proveedor que pueden ejecutarlo.",
    installing: "Instalando el plugin de lich para {name}…",
    installed: "Plugin instalado. Reinicia tus sesiones para aplicarlo.",
    installFailed: "Falló la instalación",
    updating: "Actualizando el plugin de lich para {name}…",
    updated: "Plugin actualizado. Reinicia tus sesiones para aplicarlo.",
    updateFailed: "Falló la actualización",
    checked: "Comprobado.",
    checkFailed: "Falló la comprobación: ¿tienes conexión?",
    cliMissing: "La CLI no está instalada",
    notInstalled: "Plugin no instalado",
    unsupported: "v{version}, no compatible con este lich",
    install: "Instalar",
    updateTo: "Actualizar a la v{version}",
    updateAll: "Actualizar todos",
    updatingAll: "Actualizando el plugin de lich en todas las CLI…",
    installVersion: "Instalar la v{version}",
    codexHint: "Codex: ejecuta /hooks en una sesión de Codex para confiar en los hooks del plugin.",
    crushHint:
      "Crush: informa de su id de sesión y actualiza el estado de git. No tiene evento de fin de turno, así que sus tarjetas no muestran estado y conservan su propio nombre.",
    ompHint:
      "oh-my-pi: informa de su estado, su nombre y los cambios de git. No tiene un evento de aprobación observado, así que una sesión que espera tu permiso muestra un indicador de carga en lugar de una campana.",
    antigravityHint:
      "Antigravity: informa de su estado, su nombre y los cambios de git. No tiene un evento de aprobación observado, así que una sesión que espera tu permiso muestra un indicador de carga en lugar de una campana.",
    cursorHint:
      "Cursor CLI: ejecuta el plugin instalado en Claude Code, así que informa de lo que informa esa versión y se actualiza con ella. lich solo añade aquí sus propias herramientas. Igual que los dos anteriores, no genera un evento de aprobación, así que una sesión que espera tu permiso muestra un indicador de carga en lugar de una campana.",
    checkForUpdates: "Buscar actualizaciones",
  },
  helpSettings: {
    bugTitle: "Informar de un error",
    bugSearchWords: "issue github incidencia bug",
    bugDescription:
      "Abre el formulario de errores en tu navegador con la versión y la plataforma ya rellenadas. lich no crea el issue: lo escribes tú. Ejecuta `lich rage` en una terminal para empaquetar el log, las versiones y lo que lich encontró en esta máquina en un único archivo que adjuntar.",
    openBugReport: "Abrir informe de error",
    logTitle: "Archivo de log",
    logSearchWords: "depuración registros debug logs",
    logDescription:
      "El log contiene rutas absolutas, nombres de proyectos y ramas, y tu login de gh, nunca un token de sesión. Léelo antes de adjuntarlo a cualquier cosa.",
    openingLogFolder: "Abriendo la carpeta del log…",
    logFolderOpened: "Carpeta del log abierta.",
    openLogFolderFailed: "No se pudo abrir la carpeta del log",
    openLogFolder: "Abrir carpeta del log",
    noLogFile: "Solo se registra en stderr: no hay archivo de log.",
    aboutTitle: "Acerca de",
    aboutSearchWords: "versión licencia version license",
    aboutVersion: "lich {version} · {platform}.",
    repository: "Repositorio",
  },
} satisfies Shape<typeof en>

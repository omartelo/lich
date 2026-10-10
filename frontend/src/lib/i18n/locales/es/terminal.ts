import type { Shape } from "../../catalog"
import type { terminal as en } from "../en/terminal"

export const terminal = {
  dropHint: {
    attachTo: "Adjuntar a {label}",
    confined: "Fuera del checkout llega como una copia",
    pasted: "Su ruta se pega en el prompt",
  },
  exitBanner: {
    restart: "Reiniciar",
    close: "Cerrar",
  },
  searchBar: {
    placeholder: "Buscar",
    label: "Buscar en la terminal",
    previous: "Coincidencia anterior",
    next: "Coincidencia siguiente",
    close: "Cerrar la búsqueda",
  },
  drop: {
    notAttached: "No adjuntado: {files}",
  },
  host: {
    stopShowing: "Dejar de mostrar {name}",
    paneName: "El panel {name}",
    rememberFailed: "No se pudo recordar la elección: {error}",
    showBeside: "Mostrar una sesión al lado de esta",
    noRoom: "No cabe otro panel",
  },
  showBesidePicker: {
    title: "Mostrar al lado",
    placeholder: "Busca sesiones para mostrar al lado…",
    search: "Buscar sesiones para mostrar al lado",
    results: "Sesiones que se pueden mostrar al lado",
    pick: "mostrar",
    noMatch: "Ninguna sesión coincide con {query}",
    onWall: "en {group}",
  },
  view: {
    restartFailed: "La sesión no se pudo reiniciar: {error}",
    startFailed: "La sesión no se pudo iniciar: {error}",
    pasteSettingFailed:
      "No se pudo leer si los pegados largos se despliegan, así que este se pegó una vez: {error}",
  },
  sessionExit: {
    ended: "La sesión terminó",
    endedWithCode: "La sesión terminó con el código {code}",
  },
  dropFiles: {
    folderConfined:
      "las carpetas fuera del checkout de esta sesión en sandbox no se pueden entregar; suelta archivos",
    folderMissing: "no se encontró la carpeta en esta sesión ni en tu home; suelta archivos",
  },
  copyToast: {
    copied: {
      one: "se copió {count} carácter al portapapeles",
      other: "se copiaron {count} caracteres al portapapeles",
    },
  },
} satisfies Shape<typeof en>

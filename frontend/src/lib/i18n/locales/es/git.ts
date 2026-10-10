import type { Shape } from "../../catalog"
import type { git as en } from "../en/git"

export const git = {
  baseStatus: {
    behind: {
      one: "{count} commit por detrás de {base}",
      other: "{count} commits por detrás de {base}",
    },
    conflict: { one: "Conflictos en {count} archivo", other: "Conflictos en {count} archivos" },
  },
  fileSearch: {
    cutMatches: "{count}+ coincidencias",
    summary: "{matches} en {files}",
    matches: { one: "{count} coincidencia", other: "{count} coincidencias" },
    cutNote: "Se muestran las primeras {count} líneas que coinciden. Acota la búsqueda.",
    tooLarge: {
      one: "{count} archivo de más de 1 MB sin buscar.",
      other: "{count} archivos de más de 1 MB sin buscar.",
    },
  },
  fileTree: {
    hidden: "Ocultos: {names}.",
    cut: "Esta carpeta tiene más archivos de los que el árbol puede listar.",
  },
  lastTurn: {
    saidPreviousTurn: "del turno anterior",
    noTurnWindow:
      "{name} no informa ni del inicio ni del final de un turno, así que no hay una ventana que acotar.",
  },
  carry: {
    reused:
      "{name} ya existía, así que se hizo checkout tal como está: el trabajo sin confirmar no se trasladó.",
    failed: "No se pudo trasladar el trabajo sin confirmar: {error}",
  },
  codemirror: {
    expand: {
      one: "Expandir {count} línea sin cambios",
      other: "Expandir {count} líneas sin cambios",
    },
    showLines: "Mostrar líneas {from}–{to}",
    revertTitle: "Revertir este cambio",
    revert: "Revertir",
  },
} satisfies Shape<typeof en>

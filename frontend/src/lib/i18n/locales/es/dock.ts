import type { Shape } from "../../catalog"
import type { dock as en } from "../en/dock"

export const dock = {
  rightDock: {
    fileBrowser: "Explorador de archivos",
    reviewChanges: "Revisar cambios",
    tabCode: "Código",
    tabReview: "Revisión",
    exitFullScreen: "Salir de pantalla completa",
    fullScreen: "Pantalla completa",
    closePanel: "Cerrar panel",
    fileTree: "El árbol de archivos",
    reviewPanel: "El panel de revisión",
    resizePanel: "Cambiar el tamaño del panel",
  },
  searchResults: {
    prompt: "Busca el texto de cada archivo de este checkout",
    searching: "Buscando…",
    noMatch: "Ningún archivo contiene “{query}”",
  },
  filesPanel: {
    searchPlaceholder: "Buscar en archivos",
    filterPlaceholder: "Filtrar por nombre",
    searchLabel: "Buscar en archivos",
    filterLabel: "Filtrar archivos por nombre",
    matchesLabel: "Lo que coincide con el campo",
    byName: "Nombre",
    byText: "Texto",
    readFailed: "No se pudo leer esta carpeta",
    loading: "Cargando…",
    noMatch: "Ningún archivo coincide",
    empty: "No hay archivos aquí",
    backToTree: "Volver al árbol de archivos",
    readOnly: "solo lectura",
  },
} satisfies Shape<typeof en>

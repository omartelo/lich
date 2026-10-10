import type { Shape } from "../../catalog"
import type { env as en } from "../en/env"

export const env = {
  providerSummary: {
    installed: "Instalado",
    installedCustomPath: "Instalado · ruta personalizada",
    signedOut: "Sin sesión iniciada",
    windowPercent: "{percent}% de la ventana de {length}",
    percentUsed: "{percent}% usado",
  },
  quota: {
    tokenLogin: "Inicio de sesión con token",
    weekShort: "sem",
  },
  paths: {
    cwdUnknown: "cwd desconocido · dentro de {host}",
  },
  sandbox: {
    windowsReason: "lich no tiene un backend de sandbox en Windows",
    windowsAdvice: "No hay nada que instalar: cada sesión corre en la máquina.",
    macReason: "sandbox-exec no está disponible",
    macAdvice:
      "macOS incluye /usr/bin/sandbox-exec, así que una máquina sin uno que funcione está rota de una manera que lich no puede reparar. Cada sesión corre en la máquina.",
    linuxReason: "bubblewrap no está instalado",
    linuxAdvice:
      "Instala bubblewrap y vuelve a abrir lich. Hasta entonces, cada sesión corre en la máquina.",
    confinedMeans:
      "Un home vacío que solo contiene el estado propio del agente, la máquina en solo lectura y escritura únicamente dentro de su checkout. La red sigue activa.",
  },
  vcsTools: {
    gitWithout: "Las ramas, los diffs y las worktrees quedan vacíos sin él.",
    ghWithout:
      "Los pull requests, las comprobaciones y los checkouts de PR no están disponibles sin él.",
  },
  binary: {
    parkedProject: "anulación de {name} desactivada",
    parkedProjectFallback: "proyecto",
    parkedGlobal: "anulación global desactivada",
    executable: "ejecutable",
    noSuchFile: "no existe el archivo",
    notOnPath: "no está en $PATH",
    notExecutable: "no es ejecutable",
    homeNotExpanded: "~ no se expande",
    relativePath: "ruta relativa",
    detailHomeShortcut:
      "lich lanza el binario directamente, así que ~ se toma literalmente en lugar de expandirse. Usa la ruta completa.",
    detailRelative:
      "Una ruta relativa se resuelve contra el directorio de trabajo de cada sesión, así que nombra un binario distinto por sesión. Usa una ruta completa.",
    detailBroken:
      "Las sesiones no arrancarán hasta que esto se corrija. Una anulación también se puede desactivar o borrar, y entonces se usa la capa de abajo.",
  },
  commitIdentity: {
    noneLead: "No hay identidad de git en este checkout.",
    noneNote: "Los commits se rechazarán hasta que se defina user.email.",
    landAsNamed: "Los commits se registran como {name}",
    landAs: "Los commits se registran como",
    setLocal: "· definido en este repositorio, anula el global.",
    setGlobal: "· git user.email, no esta cuenta.",
  },
} satisfies Shape<typeof en>

import type { Shape } from "../../catalog"
import type { update as en } from "../en/update"

export const update = {
  plugin: {
    incompatibleInstall:
      "Este lich no es compatible con el plugin de lich de {installs}. Instala la v{version}, la versión que este lich admite.",
    incompatibleUpdate:
      "Este lich no es compatible con el plugin de lich de {installs}. Actualiza lich, o comprueba que tienes conexión para encontrar una versión que este lich admita.",
  },
  progress: {
    updateFailed: "Falló la actualización: {error}",
    installFailed: "Falló la instalación: {error}",
    downloadFailed: "Falló la descarga: {error}",
    downloadFailedAt: "Falló la descarga al {percent}%: {error}",
    bytesOf: "{received} de {total}",
  },
} satisfies Shape<typeof en>

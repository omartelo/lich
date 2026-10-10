import type { Shape } from "../../catalog"
import type { update as en } from "../en/update"

export const update = {
  plugin: {
    incompatibleInstall:
      "O plugin do lich em {installs} não é compatível com este lich. Instale a v{version}, a versão que este lich suporta.",
    incompatibleUpdate:
      "O plugin do lich em {installs} não é compatível com este lich. Atualize o lich ou verifique se você está online para encontrar uma versão que este lich suporte.",
  },
  progress: {
    updateFailed: "Falha na atualização: {error}",
    installFailed: "Falha na instalação: {error}",
    downloadFailed: "Falha no download: {error}",
    downloadFailedAt: "Falha no download em {percent}%: {error}",
    bytesOf: "{received} de {total}",
  },
} satisfies Shape<typeof en>

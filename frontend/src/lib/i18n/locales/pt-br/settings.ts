import type { Shape } from "../../catalog"
import type { settings as en } from "../en/settings"

export const settings = {
  language: {
    uiTitle: "Idioma da interface",
    uiDescription: "O idioma das telas do próprio lich.",
    promptTitle: "Idioma dos prompts",
    promptDescription:
      "O idioma do texto que o lich entrega aos seus agentes: instruções, mensagens repassadas e avisos. Sessões iniciadas a partir de agora usam ele em tudo; as já abertas recebem ele só nas mensagens repassadas.",
    promptSaveFailed: "Não foi possível salvar o idioma dos prompts: {error}",
  },
} satisfies Shape<typeof en>

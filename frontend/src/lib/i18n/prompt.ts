import { promptLanguageStore } from "@/lib/prompt-language-store"
import type { At, ParamsArg } from "./catalog"
import { tIn } from "./i18n"
import type { MessageKey, Messages } from "./locales/en"

type PromptKey = Extract<MessageKey, `prompts.${string}`>

/** The `prompts` message under key, in the prompt language: the one the text
 * lich types into an agent is written in, independent of the interface's. */
export function prompt<K extends PromptKey>(
  key: K,
  ...params: ParamsArg<At<Messages, K>, string | number>
): string {
  return tIn(promptLanguageStore.get(), key, ...params)
}

import { useSyncExternalStore } from "react"
import { toast } from "sonner"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { LOCALES, LOCALE_NAMES, type Locale, setLocale, useLocale, useT } from "@/lib/i18n/i18n"
import { promptLanguageStore, setPromptLanguage } from "@/lib/prompt-language-store"
import { errorText } from "@/lib/utils"
import { SettingRow } from "./SettingBlock"

// The two languages are independent on purpose: someone may read lich in
// English and talk to their agents in Portuguese.
export function LanguageSettings() {
  const t = useT()
  const uiLanguage = useLocale()
  const promptLanguage = useSyncExternalStore(
    promptLanguageStore.subscribe,
    promptLanguageStore.get,
  )

  const choosePromptLanguage = (next: Locale) => {
    setPromptLanguage(next).catch((error: unknown) =>
      toast.error(t("settings.language.promptSaveFailed", { error: errorText(error) })),
    )
  }

  return (
    <>
      <SettingRow
        title={t("settings.language.uiTitle")}
        description={t("settings.language.uiDescription")}
      >
        <LanguageSelect
          value={uiLanguage}
          ariaLabel={t("settings.language.uiTitle")}
          onChange={setLocale}
        />
      </SettingRow>
      <SettingRow
        title={t("settings.language.promptTitle")}
        description={t("settings.language.promptDescription")}
      >
        <LanguageSelect
          value={promptLanguage}
          ariaLabel={t("settings.language.promptTitle")}
          onChange={choosePromptLanguage}
        />
      </SettingRow>
    </>
  )
}

interface LanguageSelectProps {
  value: Locale
  ariaLabel: string
  onChange: (value: Locale) => void
}

function LanguageSelect({ value, ariaLabel, onChange }: LanguageSelectProps) {
  return (
    <Select
      value={value}
      items={LOCALE_NAMES}
      onValueChange={(next) => next && onChange(next as Locale)}
    >
      <SelectTrigger className="w-48 shrink-0" aria-label={ariaLabel}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {LOCALES.map((locale) => (
            <SelectItem key={locale} value={locale}>
              {LOCALE_NAMES[locale]}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

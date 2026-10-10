import {
  enabledProviders,
  refreshProviders,
  setProviderDefault,
  useDefaultProvider,
  useProviders,
} from "@/lib/providers-store"
import { CheckAgainButton } from "@/components/common/CheckAgainButton"
import { useT } from "@/lib/i18n/i18n"
import { ProviderSelect } from "./ProviderSelect"
import { ProviderToggleRow } from "./ProviderToggleRow"
import { SettingBlock } from "./SettingBlock"

// ProvidersSettings is the first-run question: which of the agents installed on
// this machine should lich offer, and which one do new sessions spawn. Enabling
// a provider offers it in New Session and gives it a row in Settings ›
// Providers, where its binary and permission rung live.
//
// Narrowed to what is on PATH, unlike the roster in Settings: here the question
// is which of *your* agents to use, so rows for agents the machine does not have
// are noise. There they are the useful state, next to the docs link that says
// how to install one.
export function ProvidersSettings() {
  const t = useT()
  const providers = useProviders()
  const defaultProvider = useDefaultProvider()

  if (providers.length === 0) {
    return (
      <p className="py-5 text-sm text-muted-foreground">{t("settings.providersPane.detecting")}</p>
    )
  }

  const listed = providers.filter((provider) => provider.installed)
  const enabled = enabledProviders(listed)

  return (
    <div className="flex flex-col">
      {enabled.length > 0 && (
        <SettingBlock
          title={t("settings.providersPane.defaultTitle")}
          description={t("settings.providersSettings.defaultDescription")}
        >
          <ProviderSelect
            providers={enabled}
            value={defaultProvider}
            ariaLabel={t("settings.providersSettings.defaultLabel")}
            onChange={setProviderDefault}
          />
        </SettingBlock>
      )}
      <div className="flex items-center justify-between pb-1.5 pt-4">
        <h2 className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          {t("settings.providersSettings.detected")}
        </h2>
        <CheckAgainButton onCheck={refreshProviders} />
      </div>
      <div className="flex flex-col divide-y divide-border">
        {listed.map((provider) => (
          <ProviderToggleRow key={provider.id} provider={provider} />
        ))}
      </div>
    </div>
  )
}

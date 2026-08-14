import { ProviderIcon } from "@/components/ProviderIcon"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { ProviderState } from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"

interface ProviderSelectProps {
  providers: ProviderState[]
  value: ProviderKind
  ariaLabel: string
  onChange: (value: ProviderKind) => void
}

export function ProviderSelect({ providers, value, ariaLabel, onChange }: ProviderSelectProps) {
  const items = Object.fromEntries(providers.map((provider) => [provider.id, provider.name]))

  return (
    <Select
      value={value}
      items={items}
      onValueChange={(next) => next && onChange(next as ProviderKind)}
    >
      <SelectTrigger className="w-64" aria-label={ariaLabel}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {providers.map((provider) => (
            <SelectItem key={provider.id} value={provider.id}>
              <span className="flex items-center gap-2">
                <ProviderIcon kind={provider.id} size={14} />
                {provider.name}
              </span>
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

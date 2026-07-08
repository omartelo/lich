import type { ReactNode } from "react"

// SettingRow is a single setting: label (and optional description) on the left,
// the control on the right, separated from the next row by a hairline divider.
// Shared by every settings section.
export function SettingRow({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-6 border-b border-border py-4">
      <div className="flex flex-col gap-0.5">
        <span className="text-sm text-foreground">{label}</span>
        {description && (
          <span className="text-xs text-muted-foreground">{description}</span>
        )}
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  )
}

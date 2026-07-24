import type { ReactNode } from "react"

// A labelled cluster of setting rows within one section, so a long section
// (Appearance holds both interface and terminal controls) reads as groups
// rather than one flat list. Sibling groups are separated by the section's
// own divide-y; blocks inside a group get their own hairlines.
export function SettingGroup({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <section className="pt-8 first:pt-0">
      <h2 className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </h2>
      <div className="divide-y divide-border">{children}</div>
    </section>
  )
}

// The single layout for every settings row, so all sections read the same.
export function SettingBlock({
  icon,
  title,
  description,
  children,
}: {
  icon?: ReactNode
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <section className="py-5">
      <div className="mb-3">
        <div className="flex items-center gap-2 text-sm font-medium text-foreground">
          {icon}
          <span>{title}</span>
        </div>
        {description && (
          <p className="mt-1 max-w-prose text-xs text-muted-foreground">
            {description}
          </p>
        )}
      </div>
      {children}
    </section>
  )
}

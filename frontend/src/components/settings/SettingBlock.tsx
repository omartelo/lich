import type { ReactNode } from "react"

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

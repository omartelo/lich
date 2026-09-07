import type { ReactNode } from "react"
import { cn } from "@/lib/utils"
import { GroupProvider, useHighlight } from "./setting-highlight"

// A labelled cluster of setting rows within one section, so a long section
// (Appearance holds both interface and terminal controls) reads as groups
// rather than one flat list. Sibling groups are separated by the section's
// own divide-y; blocks inside a group get their own hairlines.
export function SettingGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section className="pt-8 first:pt-0">
      <h2 className="mb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </h2>
      {/* The group is what tells two blocks with the same title apart, so the
          search's highlight needs to know which one it is standing in. */}
      <GroupProvider value={label}>
        <div className="divide-y divide-border">{children}</div>
      </GroupProvider>
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
  // Lit when the search sent the user here: a fill that fades out on its own,
  // in the same accent every selection in the app uses.
  const { lit, ref } = useHighlight<HTMLElement>(title)

  return (
    <section
      ref={ref}
      className={cn(
        "py-5 transition-colors duration-700",
        lit && "-mx-3 rounded-lg bg-accent/60 px-3",
      )}
    >
      <div className="mb-3">
        <div className="flex items-center gap-2 text-sm font-medium text-foreground">
          {icon}
          <span>{title}</span>
        </div>
        {description && (
          <p className="mt-1 max-w-prose text-xs text-muted-foreground">{description}</p>
        )}
      </div>
      {children}
    </section>
  )
}

// A setting that fits on one line: the name on the left, the control on the
// right, hairline between rows. What Sandbox and Hotkeys already draw by hand,
// and what Appearance moved to when its controls stopped needing a paragraph
// each. A block is still the right shape for a control that carries an editor
// or a long explanation; this is for the rest.
export function SettingRow({
  title,
  description,
  children,
}: {
  title: string
  description?: ReactNode
  children: ReactNode
}) {
  const { lit, ref } = useHighlight<HTMLElement>(title)

  return (
    <section
      ref={ref}
      className={cn(
        "flex items-center justify-between gap-6 border-t border-border py-3 transition-colors duration-700 first:border-t-0",
        lit && "-mx-3 rounded-lg bg-accent/60 px-3",
      )}
    >
      <div className="min-w-0">
        <div className="text-sm font-medium text-foreground">{title}</div>
        {description && <div className="mt-0.5 text-xs text-muted-foreground">{description}</div>}
      </div>
      <div className="flex shrink-0 items-center gap-2">{children}</div>
    </section>
  )
}

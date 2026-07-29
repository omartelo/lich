import type { CSSProperties } from "react"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { useSettings } from "@/providers/settings"

/**
 * App toast host. Wraps sonner's Toaster, driving its theme from lich's
 * settings (the app toggles a `.dark` class rather than using next-themes) and
 * mapping colors to the popover tokens so toasts match tooltips and menus.
 */
export function Toaster(props: ToasterProps) {
  const { resolvedTheme } = useSettings()
  return (
    <Sonner
      theme={resolvedTheme.scheme}
      position="top-right"
      // Drop the stack below the top bar (h-10 = 2.5rem) so a toast never
      // lands on the notification and settings buttons in the corner.
      offset={{ top: "3rem", right: "1rem" }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as CSSProperties
      }
      {...props}
    />
  )
}

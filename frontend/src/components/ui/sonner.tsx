import type { CSSProperties } from "react"
import { Toaster as Sonner, type ToasterProps } from "sonner"
import { useSettings } from "@/lib/settings"

/**
 * App toast host. Wraps sonner's Toaster, driving its theme from skipo's
 * settings (the app toggles a `.dark` class rather than using next-themes) and
 * mapping colors to the popover tokens so toasts match tooltips and menus.
 */
export function Toaster(props: ToasterProps) {
  const { resolvedTheme } = useSettings()
  return (
    <Sonner
      theme={resolvedTheme}
      position="top-right"
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

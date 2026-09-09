import { useState } from "react"
import { toast } from "sonner"
import { useSettings } from "@/providers/settings"
import { useCostReadout, useCostReadoutReady } from "@/lib/use-cost-readout"
import { setCostReadout } from "@/lib/cost-readout-store"
import {
  DEFAULT_FOOTER_LAYOUT,
  hasFooterItem,
  resolveFooterLayout,
  type FooterLayout,
} from "@/lib/footer-layout"
import { errorText } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { SettingRow } from "./SettingBlock"
import { FooterLayoutEditor, FooterLayoutPreview } from "./FooterLayoutEditor"

export function FooterSettings() {
  const { footerLayout, setFooterLayout, footerVisibility, showContextUsage } = useSettings()
  const showCost = useCostReadout()
  const ready = useCostReadoutReady()
  const [saving, setSaving] = useState(false)
  const layout = resolveFooterLayout(footerLayout, footerVisibility, showContextUsage, showCost)
  const change = async (next: FooterLayout) => {
    if (!ready || saving) return
    setSaving(true)
    try {
      const cost = hasFooterItem(next, "cost")
      if (showCost !== cost) await setCostReadout(cost)
      setFooterLayout(next)
    } catch (error) {
      toast.error(`Could not save the footer: ${errorText(error)}`)
    } finally {
      setSaving(false)
    }
  }
  return (
    <>
      {/* Row, editor and preview are one setting, so they are one child of the
          section's divide-y: as separate children the hairline lands between
          the row and the editor, drawn tight against the zone headings. */}
      <div>
        {/* The row names the setting and holds the one control that is not a
            drag; the editor below it is always open, because it is the thing the
            row is about and hiding it behind a click buys nothing. */}
        <SettingRow
          title="Footer"
          description={
            !ready
              ? "Loading saved choices…"
              : saving
                ? "Saving…"
                : "Drag an item between the sides, or out to hide it. Applies to every project."
          }
        >
          <Button
            variant="ghost"
            size="xs"
            disabled={!ready || saving}
            onClick={() => void change(DEFAULT_FOOTER_LAYOUT)}
          >
            Restore default
          </Button>
        </SettingRow>
        <FooterLayoutEditor
          layout={layout}
          disabled={!ready || saving}
          onChange={(next) => void change(next)}
        />
        <FooterLayoutPreview layout={layout} />
      </div>
      {hasFooterItem(layout, "cost") && <SpendCeiling />}
    </>
  )
}

function SpendCeiling() {
  const { costBudget, setCostBudget } = useSettings()
  // Preserve a half-typed decimal while the stored amount stays numeric.
  const [budget, setBudget] = useState(() => (costBudget > 0 ? String(costBudget) : ""))
  return (
    <SettingRow
      title="Spend ceiling"
      description="Warn as the session's API cost nears this amount: amber at 80%, red at 95%. Empty for none."
    >
      <Input
        type="number"
        min={0}
        step={1}
        value={budget}
        onChange={(event) => {
          setBudget(event.target.value)
          setCostBudget(Number(event.target.value))
        }}
        placeholder="No ceiling"
        aria-label="Session spend ceiling in dollars"
        className="w-40 font-mono"
      />
    </SettingRow>
  )
}

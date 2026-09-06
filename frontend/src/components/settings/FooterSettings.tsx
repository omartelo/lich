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
import { SettingBlock, SettingGroup } from "./SettingBlock"
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
    <SettingGroup label="Footer">
      <SettingBlock
        title="Footer layout"
        description="Drag items between the rows to show, hide or reorder them, or use an item's menu. Applies to every project and provider."
      >
        <div className="mb-3 flex items-center justify-between gap-3">
          <span className="text-xs text-muted-foreground">
            {!ready
              ? "Loading saved choices…"
              : saving
                ? "Saving…"
                : "Readings appear when the provider reports them."}
          </span>
          <Button
            variant="ghost"
            size="xs"
            disabled={!ready || saving}
            onClick={() => void change(DEFAULT_FOOTER_LAYOUT)}
          >
            Restore default
          </Button>
        </div>
        <FooterLayoutEditor
          layout={layout}
          disabled={!ready || saving}
          onChange={(next) => void change(next)}
        />
        <FooterLayoutPreview layout={layout} />
      </SettingBlock>
      {hasFooterItem(layout, "cost") && <SpendCeiling />}
    </SettingGroup>
  )
}

function SpendCeiling() {
  const { costBudget, setCostBudget } = useSettings()
  // Preserve a half-typed decimal while the stored amount stays numeric.
  const [budget, setBudget] = useState(() => (costBudget > 0 ? String(costBudget) : ""))
  return (
    <SettingBlock
      title="Spend ceiling"
      description="Warn as the session's API cost approaches this amount: amber at 80%, red at 95%. This does not stop the session or represent subscription charges. Leave empty for none."
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
    </SettingBlock>
  )
}

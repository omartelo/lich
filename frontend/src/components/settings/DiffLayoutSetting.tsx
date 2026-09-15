import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { DIFF_LAYOUTS, type DiffLayout } from "@/lib/git/diff-layout"
import { useSettings } from "@/providers/settings"
import { SettingRow } from "./SettingBlock"

const LABELS: Record<DiffLayout, string> = { unified: "Unified", split: "Side-by-side" }

// DiffLayoutSetting picks how every diff draws, the Review tab's and a pull
// request's. It is the only place the choice is made: the panels draw what was
// chosen and carry no switch of their own.
export function DiffLayoutSetting() {
  const { diffLayout, setDiffLayout } = useSettings()
  return (
    <SettingRow
      title="Diff layout"
      description="How changes are drawn in the Review tab and in pull requests. Side-by-side shows unified while the panel is too narrow for two columns."
    >
      <ToggleGroup
        value={[diffLayout]}
        onValueChange={(next) => next[0] && setDiffLayout(next[0] as DiffLayout)}
        spacing={1}
        aria-label="Diff layout"
        className="shrink-0 border border-border p-[0.1875rem]"
      >
        {DIFF_LAYOUTS.map((layout) => (
          <ToggleGroupItem key={layout} value={layout} size="sm">
            {LABELS[layout]}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </SettingRow>
  )
}

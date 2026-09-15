// The two ways a diff can be drawn, chosen once in Settings › Version Control.
export const DIFF_LAYOUTS = ["unified", "split"] as const
export type DiffLayout = (typeof DIFF_LAYOUTS)[number]

// Below this card width each side of a split diff holds under ~35 columns of
// code, and nearly every line scrolls. Pixels, not rem: CodeMirror sets its own
// 12px text, which the app's zoom does not scale, so the columns that fit a
// width do not change with it either.
export const SPLIT_MIN_WIDTH_PX = 560

// shownLayout is what a card draws for the chosen layout: a split that does not
// fit falls back to unified until the card widens again. `fits` is null before
// the card has been measured, which draws unified too.
export function shownLayout(chosen: DiffLayout, fits: boolean | null): DiffLayout {
  return chosen === "split" && fits === true ? "split" : "unified"
}

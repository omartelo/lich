import type { Session } from "./sessions"

// The colours a card can be painted with. Fixed rather than theme tokens: a new
// app token is required by every published theme pack on import, so the palette
// stays out of the theme and is mixed into its surfaces instead (TINTED_FILL).
// Values are Tailwind's 500 step, which sits mid-lightness and so tints the
// light and the dark sidebar alike.
export const CARD_COLORS = {
  red: "oklch(0.637 0.237 25.331)",
  orange: "oklch(0.705 0.213 47.604)",
  amber: "oklch(0.769 0.188 70.08)",
  green: "oklch(0.723 0.219 149.579)",
  teal: "oklch(0.704 0.14 182.503)",
  blue: "oklch(0.623 0.214 259.815)",
  violet: "oklch(0.606 0.25 292.717)",
  pink: "oklch(0.656 0.241 354.308)",
} as const

export type CardColor = keyof typeof CARD_COLORS

export const CARD_COLOR_NAMES = Object.keys(CARD_COLORS) as CardColor[]

export function isCardColor(name: string | undefined): name is CardColor {
  return name !== undefined && Object.prototype.hasOwnProperty.call(CARD_COLORS, name)
}

// The card's fill per state, each one mixing more of --card-tint than the last
// so a selected card reaches the fullest colour. The steps sit where the theme's
// own fills do (hover accent/60, showing accent/55, active accent), and stop at
// 38% because the muted second line starts losing contrast on the light theme
// past ~45%. Mixed in oklab, not oklch: the theme's greys carry a hue of their
// own, and oklch would interpolate toward it, turning a selected teal card blue.
export const TINTED_FILL = {
  rest: "bg-[color-mix(in_oklab,var(--card-tint)_10%,transparent)]",
  hover: "hover:bg-[color-mix(in_oklab,var(--card-tint)_22%,var(--accent))]",
  showing: "bg-[color-mix(in_oklab,var(--card-tint)_26%,var(--accent))]",
  active: "bg-[color-mix(in_oklab,var(--card-tint)_38%,var(--accent))]",
}

// The colour every session in a folder shares, or undefined when they differ or
// none has one. A folder has no colour of its own (painting it paints its
// cards), so this is what its header and its menu read back.
export function sharedColor(sessions: readonly Session[]): CardColor | undefined {
  const first = sessions[0]?.color
  if (!isCardColor(first)) {
    return undefined
  }
  return sessions.every((session) => session.color === first) ? first : undefined
}

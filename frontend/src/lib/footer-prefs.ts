import { readPref } from "./prefs"

export interface FooterVisibility {
  model: boolean
  plan: boolean
  handsOn: boolean
  clock: boolean
}

export const DEFAULT_FOOTER_VISIBILITY: FooterVisibility = {
  model: true,
  plan: true,
  handsOn: true,
  clock: false,
}

const KEY = "lich.footer.visibility"

// Model and context used to share a switch. Preserve that choice until the
// user selects model visibility independently in Appearance.
export function parseFooterVisibility(
  raw: string | null,
  legacyContext: boolean,
): FooterVisibility {
  const fallback = { ...DEFAULT_FOOTER_VISIBILITY, model: legacyContext }
  try {
    const value: unknown = JSON.parse(raw ?? "null")
    if (!value || typeof value !== "object" || Array.isArray(value)) return fallback
    const stored = value as Record<string, unknown>
    for (const key of Object.keys(fallback) as (keyof FooterVisibility)[]) {
      if (typeof stored[key] === "boolean") fallback[key] = stored[key]
    }
    return fallback
  } catch {
    return fallback
  }
}

export const readFooterVisibility = (legacyContext: boolean): FooterVisibility =>
  parseFooterVisibility(readPref(KEY), legacyContext)

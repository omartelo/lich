import { getLocale } from "./i18n"

type Unit = "second" | "minute" | "hour" | "day" | "megabyte"

// Callers format every second (the sidebar clocks) or per row, so the
// formatters are built once per language, unit and style.
const formats = new Map<string, Intl.NumberFormat>()

/** value with its unit in the current language: "5h" in English, "5 h" in
 * Portuguese. Narrow is the width a status bar has; short keeps the space a
 * figure like "10.5 MB" wants. */
export function formatUnit(
  value: number,
  unit: Unit,
  display: "narrow" | "short" = "narrow",
  fractionDigits = 0,
): string {
  const locale = getLocale()
  const key = `${locale}:${unit}:${display}:${fractionDigits}`
  let format = formats.get(key)
  if (!format) {
    format = new Intl.NumberFormat(locale, {
      style: "unit",
      unit,
      unitDisplay: display,
      minimumFractionDigits: fractionDigits,
      maximumFractionDigits: fractionDigits,
    })
    formats.set(key, format)
  }
  return format.format(value)
}

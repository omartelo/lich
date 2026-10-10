import { getLocale, t } from "@/lib/i18n/i18n"

// How long ago something happened, at the coarsest unit that still says
// something. Two lists read it: the pull requests, where a row has space for
// "3h" and not for a date and a time, and the closed sessions in the palette,
// where the same is true and the scale runs further back.
//
// One vocabulary rather than two, so "3d" means the same thing wherever the
// window shows it. The words are Intl's, in the interface language.

const MINUTE_MS = 60_000

type AgoUnit = "minute" | "hour" | "day" | "week"

// elapsed splits milliseconds into a single unit: minutes, then hours, then
// days, then weeks. Each is floored, so it never claims time that has not
// passed, and anything under a minute is null, "now": a readout that counted
// seconds would change under the eye of somebody reading a list.
//
// A negative elapsed time (a clock the machine corrected backwards) reads as
// now rather than as a negative age.
function elapsed(ms: number): { value: number; unit: AgoUnit } | null {
  const minutes = Math.floor(Math.max(0, ms) / MINUTE_MS)
  if (minutes < 1) return null
  if (minutes < 60) return { value: minutes, unit: "minute" }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return { value: hours, unit: "hour" }
  const days = Math.floor(hours / 24)
  return days < 7 ? { value: days, unit: "day" } : { value: Math.floor(days / 7), unit: "week" }
}

// agoUnit renders elapsed milliseconds as one narrow unit: "3h", "now".
export function agoUnit(ms: number): string {
  const age = elapsed(ms)
  const locale = getLocale()
  if (!age) {
    return new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(0, "second")
  }
  return new Intl.NumberFormat(locale, {
    style: "unit",
    unit: age.unit,
    unitDisplay: "narrow",
  }).format(age.value)
}

// agoLabel is agoUnit as a phrase, for the places where the unit alone would be
// ambiguous: a history row's "3d" sits where a live card shows how long its
// current state has lasted, and the two must not read alike.
//
// at is unix seconds — what the store records — and 0 means the row predates
// lich recording it, which is no date rather than 1970.
export function agoLabel(at: number, now: number = Date.now()): string {
  if (at <= 0) {
    return ""
  }
  const age = elapsed(now - at * 1000)
  if (!age) {
    return t("common.time.justNow")
  }
  return new Intl.RelativeTimeFormat(getLocale(), { style: "narrow", numeric: "always" }).format(
    -age.value,
    age.unit,
  )
}

import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"
import { t } from "@/lib/i18n/i18n"
import type { Messages } from "@/lib/i18n/locales/en"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// count renders "1 file" / "3 files" in the interface language, for the
// readouts that name how many of something a checkout has. A new noun is a
// plural message under common.count.
export function count(n: number, noun: keyof Messages["common"]["count"]): string {
  return t(`common.count.${noun}`, { count: n })
}

// errorText renders an unknown thrown value (bindings reject with anything)
// as the message a toast or dialog can show.
export function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

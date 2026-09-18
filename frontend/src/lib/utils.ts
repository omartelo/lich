import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// count renders "1 file" / "3 files" — the English plural of a countable noun,
// for the readouts that name how many of something a checkout has.
export function count(n: number, noun: string): string {
  return `${n} ${noun}${n === 1 ? "" : "s"}`
}

// errorText renders an unknown thrown value (bindings reject with anything)
// as the message a toast or dialog can show.
export function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

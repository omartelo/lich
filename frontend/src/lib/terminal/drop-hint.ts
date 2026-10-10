import { t } from "@/lib/i18n/i18n"
// The line under "Attach to <session>" while a file drag is over a terminal:
// what the drop does. A confined session (internal/sandbox) cannot see the
// user's file, so for it the honest line is the copy.
export function dropHintDetail(confined: boolean): string {
  return confined ? t("terminal.dropHint.confined") : t("terminal.dropHint.pasted")
}

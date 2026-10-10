import { Tray } from "@/lib/rpc"
import { subscribeLocale, t } from "@/lib/i18n/i18n"

// The tray menu is drawn by the backend, but its words are the interface
// language's, which lives here: the page hands them over at load and again on
// every language change, and the tray comes up with the first of them.
function pushTrayLabels(): void {
  void Tray.SetLabels({
    show: t("shell.tray.show"),
    // The template, placeholder and all: the backend fills the count in as it
    // changes, long after this page may have closed.
    running: t("shell.tray.running", { count: "{count}" }),
    quit: t("shell.tray.quit"),
  }).catch(() => {})
}

// syncTrayLabels pushes the labels now and on each language change, and
// returns the unsubscribe.
export function syncTrayLabels(): () => void {
  pushTrayLabels()
  return subscribeLocale(pushTrayLabels)
}

import { OSC8LinkProvider, UrlRegexProvider } from "ghostty-web"
import type { ILink, ILinkProvider, Terminal as Ghostty } from "ghostty-web"
import { System } from "./rpc"

// registerLinkOpening wires ghostty-web's built-in link detection — OSC 8
// hyperlinks and URL-regex matches — and opens a match in the user's real
// browser on Ctrl/Cmd-click. Without this no provider is registered, so links
// are never detected (no hover underline, no click). The built-in providers
// detect the same links but open them with window.open, which the WebKitGTK
// webview traps instead of handing to the desktop; System.OpenExternal routes
// the URL through the backend to the OS default browser in any shell.
export function registerLinkOpening(term: Ghostty): void {
  const detectors: ILinkProvider[] = [new OSC8LinkProvider(term), new UrlRegexProvider(term)]
  for (const detector of detectors) {
    term.registerLinkProvider({
      provideLinks(y, callback) {
        detector.provideLinks(y, (links) => callback(links?.map(openInBrowser)))
      },
      dispose: () => detector.dispose?.(),
    })
  }
}

// openInBrowser returns a copy of a detected link whose activation opens the URL
// in the OS browser via Wails on Ctrl/Cmd-click, matching a real terminal's
// modifier-click-to-open behaviour.
export function openInBrowser(link: ILink): ILink {
  return {
    ...link,
    activate: (event: MouseEvent) => {
      if (event.ctrlKey || event.metaKey) {
        void System.OpenExternal(link.text)
      }
    },
  }
}

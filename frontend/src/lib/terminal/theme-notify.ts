/**
 * Color palette update notifications, private mode 2031
 * (https://contour-terminal.org/vt-extensions/color-palette-update-notifications/).
 *
 * An app that follows the terminal's light or dark look (Claude Code under its
 * "auto" theme) asks for the background once (OSC 11) and then turns this mode
 * on to hear about a change. xterm.js ignores the mode, so without it the app
 * keeps whatever it decided at boot: a session born under a dark lich theme
 * draws dark-theme diffs on a light terminal for the rest of its life.
 */

import type { IDisposable, Terminal } from "@xterm/xterm"
import type { ThemeDefinition } from "@/lib/api-types"

type Scheme = ThemeDefinition["scheme"]

const THEME_NOTIFY_MODE = 2031
const COLOR_SCHEME_QUERY = 996
// DECRPM status values: 1 set, 2 reset.
const MODE_SET = 1
const MODE_RESET = 2

/** The report the mode sends on a change and DSR 996 answers with. */
export function colorSchemeReport(scheme: Scheme): string {
  return `\x1b[?997;${scheme === "dark" ? 1 : 2}n`
}

/**
 * Teaches `term` the mode: DECSET/DECRST 2031 flip `state.themeNotify`, and the
 * DSR 996 query and a DECRQM for 2031 are answered through `reply`, the
 * scheme read at that moment. Every other private mode and query is left to
 * xterm.
 */
export function watchThemeNotify(
  term: Terminal,
  state: { themeNotify: boolean },
  scheme: () => Scheme,
  reply: (data: string) => void,
): IDisposable {
  const names = (params: (number | number[])[]) => params.includes(THEME_NOTIFY_MODE)
  const handlers = [
    term.parser.registerCsiHandler({ prefix: "?", final: "h" }, (params) => {
      state.themeNotify ||= names(params)
      return false
    }),
    term.parser.registerCsiHandler({ prefix: "?", final: "l" }, (params) => {
      state.themeNotify &&= !names(params)
      return false
    }),
    term.parser.registerCsiHandler({ prefix: "?", final: "n" }, (params) => {
      if (params.length !== 1 || params[0] !== COLOR_SCHEME_QUERY) {
        return false
      }
      reply(colorSchemeReport(scheme()))
      return true
    }),
    term.parser.registerCsiHandler({ prefix: "?", intermediates: "$", final: "p" }, (params) => {
      if (params.length !== 1 || params[0] !== THEME_NOTIFY_MODE) {
        return false
      }
      reply(`\x1b[?${THEME_NOTIFY_MODE};${state.themeNotify ? MODE_SET : MODE_RESET}$y`)
      return true
    }),
  ]
  return {
    dispose() {
      for (const handler of handlers) {
        handler.dispose()
      }
    },
  }
}

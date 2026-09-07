import { createContext, useContext, useEffect, useRef } from "react"

/** The control the settings search just sent the user to. The group is part of
 * it because titles repeat: Appearance holds two blocks called Theme, and
 * lighting both up answers the question the path in the result had just
 * settled. Empty group means "wherever it is", which is what a result that
 * walked into a provider's own screen needs. */
export interface HighlightTarget {
  title: string
  group: string
}

const NOTHING: HighlightTarget = { title: "", group: "" }

// A context rather than a prop: the blocks sit three or four components below
// the screen, in panes that know nothing about searching, and threading a
// target through all of them would put the search in every signature.
const HighlightContext = createContext<HighlightTarget>(NOTHING)

// The group a block is rendered inside, provided by SettingGroup and by the
// hotkey pane's own grouping.
const GroupContext = createContext("")

export const HighlightProvider = HighlightContext.Provider
export const GroupProvider = GroupContext.Provider

/** Whether this control is the one that was searched for, plus the ref to hang
 * on it so it can be scrolled into view.
 *
 * Matched by prefix because one block titles itself after the open project
 * ("GitHub account for lich"), and the index holds the part that does not
 * change. */
export function useHighlight<T extends HTMLElement>(title: string) {
  const target = useContext(HighlightContext)
  const group = useContext(GroupContext)
  const named = target.title !== "" && title.toLowerCase().startsWith(target.title.toLowerCase())
  const lit = named && (target.group === "" || target.group === group)
  const ref = useRef<T>(null)

  useEffect(() => {
    if (lit) {
      ref.current?.scrollIntoView({ block: "center", behavior: "smooth" })
    }
  }, [lit])

  return { lit, ref }
}

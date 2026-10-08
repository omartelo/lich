import { flushSync } from "react-dom"

// The names the sidebar's two shapes share, so the browser can fly each glyph
// from where the rail draws it to where the expanded sidebar does. A name must
// be unique in the document at the moment of the snapshot, which holds because
// the rail and the sidebar are never mounted together.
export const SIDEBAR_MORPH = {
  panel: "sidebar-panel",
  // The stage and the footer slide with the sidebar's edge rather than
  // crossfading in place (index.css keeps their snapshots unscaled).
  stage: "stage",
  footer: "footer",
  toggle: "sidebar-toggle",
  newSession: "sidebar-new-session",
  // A UUID may start with a digit, which a <custom-ident> may not.
  session: (id: string) => `sidebar-session-${id}`,
}

// morph runs a React state change inside a view transition: the browser
// snapshots the page, the update commits synchronously, and every element
// carrying a view-transition-name animates from its old box to its new one.
export function morph(update: () => void): void {
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    update()
    return
  }
  document.startViewTransition(() => flushSync(update))
}

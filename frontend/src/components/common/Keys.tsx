// A key combo as printed text — the one <kbd> in the app, shared by the
// shortcuts overlay and the Hotkeys settings pane so a chord never reads two
// ways in two places.
export function Keys({ children }: { children: React.ReactNode }) {
  return (
    <kbd className="shrink-0 rounded border border-b-2 bg-muted px-1.5 py-0.5 font-mono text-[0.625rem] leading-none text-muted-foreground">
      {children}
    </kbd>
  )
}

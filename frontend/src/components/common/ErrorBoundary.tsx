import { RotateCcw, TriangleAlert } from "lucide-react"
import { Component, type ErrorInfo, type ReactNode } from "react"
import { Notice } from "@/components/common/Notice"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface ErrorBoundaryProps {
  /** Names the area that stopped rendering — the fallback is all the user is left with. */
  label: string
  children: ReactNode
  /** Clears the error when it changes: the subtree swapped in is not the one that threw. */
  resetKey?: string
  /** What the retry offers to do, where "try again" undersells it. */
  retry?: string
  /** Positioning for the fallback, whose default is to fill whatever it is handed. */
  className?: string
}

interface ErrorBoundaryState {
  error: Error | null
  /** Throws since the last subtree that rendered. Two means retrying is not it. */
  throws: number
}

// After this many consecutive throws the retry has answered the question: the
// subtree is not coming back, and offering it again is a loop on a fallback.
const RETRIES_BEFORE_RELOAD = 2

// The one thing standing between a render throw and a blank window. Nothing
// catching means React unmounts at the root, and lich's root is the window: the
// sidebar, the terminals and the footer all go with the screen that threw,
// while the sessions themselves keep running with nothing on screen saying so.
// A class is not a style choice here — React offers no hook that catches.
//
// The stage is caught too. Unmounting it used to destroy every session's xterm;
// the terminals now outlive their components (lib/terminal/terminal-registry.ts),
// so the fallback costs a repaint and nothing else.
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null, throws: 0 }

  static getDerivedStateFromError(error: Error): Pick<ErrorBoundaryState, "error"> {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`${this.props.label} failed to render`, error, info.componentStack)
    // Counted here rather than beside the error, because getDerivedStateFromError
    // is handed no previous state to add to.
    this.setState((previous) => ({ throws: previous.throws + 1 }))
  }

  componentDidUpdate(previous: ErrorBoundaryProps) {
    if (this.state.error !== null && previous.resetKey !== this.props.resetKey) {
      this.setState({ error: null, throws: 0 })
      return
    }
    // Reached with no error is the children having committed, so whatever the
    // count was, the retry worked and the next throw starts over.
    if (this.state.error === null && this.state.throws > 0) {
      this.setState({ throws: 0 })
    }
  }

  render() {
    const { error, throws } = this.state
    if (error === null) {
      return this.props.children
    }
    const exhausted = throws >= RETRIES_BEFORE_RELOAD
    return (
      <div
        className={cn(
          "flex h-full w-full flex-col items-center justify-center gap-2 bg-background p-6 text-center",
          this.props.className,
        )}
      >
        <TriangleAlert className="size-8 text-muted-foreground" />
        <p className="text-sm text-foreground">{this.props.label} stopped rendering</p>
        {/* The message, not just a console line: the window it would have been
            read in is the one this fallback is standing in for. */}
        <Notice className="max-w-md py-0 font-mono break-words">
          {error.message || String(error)}
        </Notice>
        {/* The same retry that just failed is a loop, so past the second throw
            the only offer left is the one that always works. The sessions keep
            running through it — a reload costs the page, not the PTYs. */}
        <Button
          variant="ghost"
          size="sm"
          onClick={() => (exhausted ? window.location.reload() : this.setState({ error: null }))}
        >
          <RotateCcw data-icon="inline-start" />
          {exhausted ? "Reload the window" : (this.props.retry ?? "Try again")}
        </Button>
      </div>
    )
  }
}

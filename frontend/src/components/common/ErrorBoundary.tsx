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
}

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
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(`${this.props.label} failed to render`, error, info.componentStack)
  }

  componentDidUpdate(previous: ErrorBoundaryProps) {
    if (this.state.error !== null && previous.resetKey !== this.props.resetKey) {
      this.setState({ error: null })
    }
  }

  render() {
    const { error } = this.state
    if (error === null) {
      return this.props.children
    }
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
        <Button variant="ghost" size="sm" onClick={() => this.setState({ error: null })}>
          <RotateCcw data-icon="inline-start" />
          {this.props.retry ?? "Try again"}
        </Button>
      </div>
    )
  }
}

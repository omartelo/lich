import { useState } from "react"
import type { ComponentPropsWithoutRef, KeyboardEvent } from "react"
import { ChevronRight, Pencil, Plus, Ungroup } from "lucide-react"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { ProviderState } from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"
import { cn } from "@/lib/utils"
import { SessionLaunchMenuItems } from "./SessionLaunchMenuItems"

interface SessionGroupHeaderProps {
  name: string
  // A block that never moves among the others: the pinned sessions. It has no
  // drag handle, so its title is a plain button.
  fixed: boolean
  // Whether the block can open a new session in itself — true for a checkout,
  // false for the gathered blocks, which have no directory to open one in.
  launch: boolean
  // Present on a wall's header only: renaming happens in place, and dissolving
  // takes the wall apart without touching a session in it.
  onRename?: (name: string) => void
  onDissolve?: () => void
  collapsed: boolean
  isDragging: boolean
  providers: ProviderState[]
  activatorRef: (element: HTMLElement | null) => void
  activatorProps: ComponentPropsWithoutRef<"button">
  onToggle: () => void
  onNewSession: (kind: ProviderKind | "shell") => void
}

interface SessionGroupTitleButtonProps {
  name: string
  fixed: boolean
  collapsed: boolean
  activatorRef: (element: HTMLElement | null) => void
  activatorProps: ComponentPropsWithoutRef<"button">
  onClick: () => void
}

function SessionGroupTitleButton({
  name,
  fixed,
  collapsed,
  activatorRef,
  activatorProps,
  onClick,
}: SessionGroupTitleButtonProps) {
  return (
    <button
      ref={activatorRef}
      type="button"
      {...activatorProps}
      aria-expanded={!collapsed}
      title={`${collapsed ? "Expand" : "Collapse"} ${name}`}
      onClick={onClick}
      className={cn(
        "group/collapse -ml-1 flex min-w-0 flex-1 items-center gap-1.5 rounded-sm px-1 py-0.5 text-left transition-colors hover:bg-accent/50",
        fixed ? "cursor-pointer" : "cursor-grab",
      )}
    >
      <ChevronRight
        className={cn(
          "size-3 shrink-0 text-muted-foreground/70 transition-[color,transform] group-hover/collapse:text-muted-foreground",
          !collapsed && "rotate-90",
        )}
      />
      <span className="min-w-0 truncate text-[0.65rem] font-semibold uppercase tracking-wider text-muted-foreground/70 transition-colors group-hover/collapse:text-muted-foreground">
        {name}
      </span>
      <span className="h-px flex-1 bg-border" />
    </button>
  )
}

export function SessionGroupHeader({
  name,
  fixed,
  launch,
  onRename,
  onDissolve,
  collapsed,
  isDragging,
  providers,
  activatorRef,
  activatorProps,
  onToggle,
  onNewSession,
}: SessionGroupHeaderProps) {
  const [editing, setEditing] = useState(false)

  const commit = (value: string) => {
    setEditing(false)
    const trimmed = value.trim()
    if (trimmed && trimmed !== name) {
      onRename?.(trimmed)
    }
  }

  const onEditKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      commit(event.currentTarget.value)
    }
    if (event.key === "Escape") {
      setEditing(false)
    }
  }

  if (editing) {
    return (
      <div className="flex items-center gap-1 px-1 pb-0.5 pt-1.5">
        <input
          // biome-ignore lint/a11y/noAutofocus: the field replaces the title only once renaming starts.
          autoFocus
          defaultValue={name}
          aria-label="Group name"
          onFocus={(event) => event.currentTarget.select()}
          onKeyDown={onEditKeyDown}
          onBlur={(event) => commit(event.currentTarget.value)}
          className="w-full rounded-sm bg-transparent px-1 py-0.5 text-[0.65rem] font-semibold uppercase tracking-wider text-foreground outline-none ring-1 ring-accent-foreground/30"
        />
      </div>
    )
  }

  const header = (
    <div className="flex items-center gap-1 px-1 pb-0.5 pt-1.5">
      <SessionGroupTitleButton
        name={name}
        fixed={fixed}
        collapsed={collapsed}
        activatorRef={activatorRef}
        activatorProps={activatorProps}
        onClick={() => !isDragging && onToggle()}
      />
      {launch && (
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`New session in ${name}`}
            title={`New session in ${name}`}
            render={<Button variant="ghost" size="icon-xs" />}
          >
            <Plus />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="max-w-56">
            <SessionLaunchMenuItems
              providers={providers}
              terminalLabel="New Terminal"
              onNewSession={onNewSession}
            />
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )

  // A wall is the only block with anything to say about itself. The others get
  // no menu rather than an empty one.
  if (!onRename && !onDissolve) {
    return header
  }
  return (
    <ContextMenu>
      <ContextMenuTrigger render={<div />}>{header}</ContextMenuTrigger>
      <ContextMenuContent>
        {onRename && (
          <ContextMenuItem onClick={() => setEditing(true)}>
            <Pencil />
            Rename group
          </ContextMenuItem>
        )}
        {onDissolve && (
          <ContextMenuItem onClick={onDissolve}>
            <Ungroup />
            Ungroup
          </ContextMenuItem>
        )}
      </ContextMenuContent>
    </ContextMenu>
  )
}

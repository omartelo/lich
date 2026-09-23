import { useState } from "react"
import type { ComponentPropsWithoutRef, KeyboardEvent } from "react"
import { ChevronRight, Folder, MoreHorizontal, Pencil, Plus, Ungroup } from "lucide-react"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { ProviderState } from "@/lib/providers-store"
import { type DropState, FILE_TARGET_ATTRIBUTE } from "@/lib/session/file-drag-store"
import type { ProviderKind } from "@/lib/session/sessions"
import type { SandboxAnswer } from "@/lib/use-sandbox-choice"
import { cn } from "@/lib/utils"
import { FolderLaunchMenuItems, type LaunchCheckout } from "./FolderLaunchMenuItems"
import { type RunMenuAction, SessionLaunchMenuItems } from "./SessionLaunchMenuItems"

interface SessionGroupHeaderProps {
  name: string
  // A block that never moves among the others: the pinned sessions. It has no
  // drag handle, so its title is a plain button.
  fixed: boolean
  // Whether the block can open a new session in itself: true for a checkout
  // and a folder, false for the pinned block and a wall.
  launch: boolean
  // Present on a folder's header: the checkouts its + opens a session in, since
  // a folder has no directory of its own.
  checkouts?: LaunchCheckout[]
  // Present on a wall's and a folder's header: renaming happens in place, and
  // taking the group apart leaves every session in it open.
  onRename?: (name: string) => void
  onDissolve?: () => void
  // Whether this block is a folder — a set of sessions the user filed by name,
  // gathered from any checkout. It wears a folder glyph and says how many cards
  // it holds, because neither is readable off the list once it is folded.
  folder: boolean
  // How many sessions the block holds, drawn on a folder alone: a checkout's
  // block is its cards, and a count there would only restate the list under it.
  count: number
  // The folders this block's sessions can be filed into, and the two ways to
  // file them: an existing folder by name, or a new one the dialog names. Both
  // absent on a block with nothing to move — a folder's own header, and the
  // pinned block, whose cards are filed one at a time from the card itself.
  folders?: string[]
  onFileAll?: (folder: string) => void
  onNewFolder?: () => void
  // What a folded block says about the cards it hides (collapsedMark).
  mark: "wait" | "done" | null
  // How the header answers a card being dragged (file-drag-store), and what it
  // files one dropped on it into: a folder's name, "" for a checkout.
  drop: DropState
  dropFolder: string
  collapsed: boolean
  isDragging: boolean
  providers: ProviderState[]
  // The project the launch menu opens into: it scopes the sandbox rung the menu
  // reads before it puts the confinement question.
  projectId: string
  activatorRef: (element: HTMLElement | null) => void
  activatorProps: ComponentPropsWithoutRef<"button">
  onToggle: () => void
  // path is the checkout a folder's + chose; a checkout's + opens in itself.
  onNewSession: (kind: ProviderKind | "shell", sandbox: SandboxAnswer, path?: string) => void
  // The checkout's Run entry: open its Run card, or go to the one it has.
  // Absent when the project ships no run script.
  run?: RunMenuAction
}

interface SessionGroupTitleButtonProps {
  name: string
  fixed: boolean
  folder: boolean
  count: number
  // What the block says while it is folded: a session waiting on the user
  // (amber), a finished turn nobody has read (emerald), or nothing. Folding a
  // block hides its cards' rings, and news the user folded away is still news.
  mark: "wait" | "done" | null
  collapsed: boolean
  // The dragged card is over this header, which files it on release.
  dropOver: boolean
  activatorRef: (element: HTMLElement | null) => void
  activatorProps: ComponentPropsWithoutRef<"button">
  onClick: () => void
}

function SessionGroupTitleButton({
  name,
  fixed,
  folder,
  count,
  mark,
  collapsed,
  dropOver,
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
        dropOver && "bg-tone-pass/15 hover:bg-tone-pass/15",
      )}
    >
      <ChevronRight
        className={cn(
          "size-3 shrink-0 text-muted-foreground/70 transition-[color,transform] group-hover/collapse:text-muted-foreground",
          !collapsed && "rotate-90",
        )}
      />
      {folder && <Folder className="size-3 shrink-0 text-muted-foreground/70" />}
      <span className="min-w-0 truncate text-2xs font-semibold uppercase tracking-wider text-muted-foreground/70 transition-colors group-hover/collapse:text-muted-foreground">
        {name}
      </span>
      {folder && (
        <span className="shrink-0 text-2xs font-medium tabular-nums text-muted-foreground/60">
          {count}
        </span>
      )}
      {collapsed && mark && (
        <span
          className={cn(
            "size-1.5 shrink-0 rounded-full",
            mark === "wait" ? "bg-tone-wait" : "bg-tone-pass",
          )}
        >
          {/* The dot is the whole of it on screen; the words are what a reader
              hears in its place, since a colour is not an announcement. */}
          <span className="sr-only">
            {mark === "wait" ? "a session here is waiting on you" : "an unread turn in here"}
          </span>
        </span>
      )}
      <span className="h-px flex-1 bg-border" />
    </button>
  )
}

export function SessionGroupHeader({
  name,
  fixed,
  launch,
  checkouts,
  onRename,
  onDissolve,
  folder,
  count,
  folders,
  onFileAll,
  onNewFolder,
  mark,
  drop,
  dropFolder,
  collapsed,
  isDragging,
  providers,
  projectId,
  activatorRef,
  activatorProps,
  onToggle,
  onNewSession,
  run,
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

  // Only a header that takes the card is found by the drag's hit test.
  const target =
    drop === "accepts" || drop === "over" ? { [FILE_TARGET_ATTRIBUTE]: dropFolder } : {}

  return (
    <div
      {...target}
      className={cn(
        "flex items-center gap-1 px-1 pb-0.5 pt-1.5 transition-opacity",
        drop === "refuses" && "opacity-45",
      )}
    >
      {/* Only the title swaps for the field. Replacing the whole header would
          unmount the menu the rename was chosen from, and the menu restores
          focus to a trigger that is no longer there — which lands on the fresh
          input as a blur, commits the name unchanged, and reads as a rename that
          does nothing. */}
      {editing ? (
        <input
          // biome-ignore lint/a11y/noAutofocus: the field replaces the title only once renaming starts.
          autoFocus
          defaultValue={name}
          aria-label="Group name"
          onFocus={(event) => event.currentTarget.select()}
          onKeyDown={onEditKeyDown}
          onBlur={(event) => commit(event.currentTarget.value)}
          className="min-w-0 flex-1 rounded-sm bg-transparent px-1 py-0.5 text-2xs font-semibold uppercase tracking-wider text-foreground outline-none ring-1 ring-accent-foreground/30"
        />
      ) : (
        <SessionGroupTitleButton
          name={name}
          fixed={fixed}
          folder={folder}
          count={count}
          mark={mark}
          collapsed={collapsed}
          dropOver={drop === "over"}
          activatorRef={activatorRef}
          activatorProps={activatorProps}
          onClick={() => !isDragging && onToggle()}
        />
      )}
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
            {checkouts ? (
              <FolderLaunchMenuItems
                folder={name}
                checkouts={checkouts}
                providers={providers}
                projectId={projectId}
                onNewSession={onNewSession}
              />
            ) : (
              <SessionLaunchMenuItems
                providers={providers}
                terminalLabel="New Terminal"
                projectId={projectId}
                onNewSession={onNewSession}
                run={run}
              />
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
      {/* A block with anything to say about itself says it through a button
          rather than a right-click: the block beside it carries a visible + for
          its own action, so a menu reachable only by right-click is one nobody
          finds. A checkout's block earns one too — filing its whole list at once
          is what makes a folder worth making with forty cards on screen. */}
      {(onRename || onDissolve || onFileAll) && (
        <DropdownMenu>
          <DropdownMenuTrigger
            aria-label={`Options for ${name}`}
            title={`Options for ${name}`}
            render={<Button variant="ghost" size="icon-xs" />}
          >
            <MoreHorizontal />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="max-w-56">
            {onRename && (
              <DropdownMenuItem onClick={() => setEditing(true)}>
                <Pencil />
                {folder ? "Rename folder" : "Rename group"}
              </DropdownMenuItem>
            )}
            {onDissolve && (
              <DropdownMenuItem onClick={onDissolve}>
                <Ungroup />
                Ungroup
              </DropdownMenuItem>
            )}
            {onFileAll && onNewFolder && (
              <DropdownMenuSub>
                <DropdownMenuSubTrigger>
                  <Folder />
                  Move group to folder
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent>
                  {(folders ?? []).map((target) => (
                    <DropdownMenuItem key={target} onClick={() => onFileAll(target)}>
                      <Folder />
                      <span className="truncate">{target}</span>
                    </DropdownMenuItem>
                  ))}
                  {(folders ?? []).length > 0 && <DropdownMenuSeparator />}
                  <DropdownMenuItem onClick={onNewFolder}>
                    <Plus />
                    New folder&hellip;
                  </DropdownMenuItem>
                </DropdownMenuSubContent>
              </DropdownMenuSub>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

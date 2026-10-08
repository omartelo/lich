import { SearchInput } from "@/components/common/SearchInput"
import { Label } from "@/components/ui/label"
import { cn, count } from "@/lib/utils"
import { type BaseBranch, rowValue } from "./useBaseBranch"

interface GroupProps {
  title: string
  items: ReadonlyArray<{ value: string; label: string; note?: string }>
  base: string
  onSelect: (value: string) => void
}

function Group({ title, items, base, onSelect }: GroupProps) {
  if (items.length === 0) {
    return null
  }
  return (
    <div>
      <div className="px-2 pb-1 pt-2 text-2xs font-semibold tracking-wider text-muted-foreground uppercase">
        {title} <span className="font-normal">({items.length})</span>
      </div>
      {items.map((item) => (
        <button
          key={item.value}
          type="button"
          role="option"
          aria-selected={base === item.value}
          onClick={() => onSelect(item.value)}
          className={cn(
            "flex w-full flex-col items-start rounded-md px-2 py-1.5 text-left font-mono text-xs outline-none transition-colors",
            base === item.value ? "bg-accent text-accent-foreground" : "hover:bg-accent/50",
          )}
        >
          <span className="w-full truncate">{item.label}</span>
          {item.note && (
            <span className="w-full truncate font-sans text-2xs text-muted-foreground">
              {item.note}
            </span>
          )}
        </button>
      ))}
    </div>
  )
}

interface BaseBranchPickerProps {
  picker: BaseBranch
  /** Whose uncommitted work the working-tree row carries, for its wording. */
  sourceNoun: "session" | "checkout"
}

// BaseBranchPicker draws useBaseBranch: the search, then the working-tree row,
// existing worktrees where they are offered, local and remote branches.
export function BaseBranchPicker({ picker, sourceNoun }: BaseBranchPickerProps) {
  const {
    branches,
    loadError,
    base,
    setBase,
    filter,
    onFilter,
    onSearchKeyDown,
    vis,
    noMatches,
    listRef,
  } = picker
  return (
    <div className="flex min-h-0 flex-col gap-1.5">
      <div className="flex items-center justify-between">
        <Label className="text-xs uppercase tracking-wide">Base branch</Label>
        {!branches && !loadError && (
          <span className="text-xs text-muted-foreground">Loading branches…</span>
        )}
      </div>
      <SearchInput
        value={filter}
        onChange={(e) => onFilter(e.target.value)}
        onKeyDown={onSearchKeyDown}
        placeholder="Search branches…"
        aria-label="Search base branches"
        autoComplete="off"
        spellCheck={false}
        className="font-mono"
      />
      <div
        ref={listRef}
        role="listbox"
        aria-label="Base branch"
        className="min-h-0 flex-1 overflow-y-auto rounded-md border border-input p-1"
      >
        <Group
          title={`This ${sourceNoun}`}
          items={
            vis.tree
              ? [
                  {
                    value: rowValue("tree", vis.tree.path),
                    label: `${vis.tree.branch} · working tree`,
                    note: `Same commit, plus the ${count(vis.tree.files, "file")} this ${sourceNoun} has not committed.`,
                  },
                ]
              : []
          }
          base={base}
          onSelect={setBase}
        />
        <Group
          title="Worktrees"
          items={vis.worktrees.map((wt) => ({
            value: rowValue("worktree", wt.path),
            label: wt.name,
          }))}
          base={base}
          onSelect={setBase}
        />
        <Group
          title="Local branches"
          items={vis.local.map((branch) => ({
            value: rowValue("local", branch),
            label: branch,
          }))}
          base={base}
          onSelect={setBase}
        />
        <Group
          title="Remote branches"
          items={vis.remote.map((branch) => ({
            value: rowValue("remote", branch),
            label: branch,
          }))}
          base={base}
          onSelect={setBase}
        />
        {noMatches && (
          <div className="px-2 py-6 text-center text-xs text-muted-foreground">
            {filter.trim() ? (
              <>
                No branches match{" "}
                <span className="font-mono text-foreground/80">{filter.trim()}</span>
              </>
            ) : (
              "No branches found"
            )}
          </div>
        )}
      </div>
    </div>
  )
}

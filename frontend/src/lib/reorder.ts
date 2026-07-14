// applyOrder rearranges items to match the given id order. It returns null when
// the ids do not name each item exactly once — a session closed or a project
// opened between the drag and the drop — so the caller drops a stale order
// instead of persisting one that would lose or duplicate an item. Each id
// consumes its item from the lookup, which is what makes a repeated id fail the
// check rather than clone the item it names.
export function applyOrder<T extends { id: string }>(items: T[], ids: string[]): T[] | null {
  const unclaimed = new Map(items.map((item) => [item.id, item]))
  const next: T[] = []
  for (const id of ids) {
    const item = unclaimed.get(id)
    if (item) {
      next.push(item)
      unclaimed.delete(id)
    }
  }
  return unclaimed.size === 0 && next.length === ids.length ? next : null
}

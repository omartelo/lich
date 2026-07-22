// Pure tree assembly for the file browser: git ls-files hands the frontend a
// flat, sorted list of repo-relative slash-separated paths; buildTree nests
// them for rendering. No DOM, so it runs under vitest's node environment.

export type TreeNodeType = "dir" | "file"

export interface TreeNode {
  /** Last path segment — what the row shows. */
  name: string
  /** Full repo-relative path, the id used for expand state and ReadFile. */
  path: string
  type: TreeNodeType
  /** Empty for files. */
  children: TreeNode[]
}

// buildTree nests flat paths into a directory tree, directories before files
// and each group sorted case-insensitively — the order an explorer expects,
// independent of git's byte order.
//
// ponytail: linear find per level, O(files · siblings); swap in a Map keyed by
// name if a huge monorepo ever makes this lag.
export function buildTree(paths: string[]): TreeNode[] {
  const root: TreeNode[] = []
  for (const path of paths) {
    const parts = path.split("/").filter(Boolean)
    let level = root
    let prefix = ""
    parts.forEach((name, i) => {
      prefix = prefix ? `${prefix}/${name}` : name
      const type: TreeNodeType = i === parts.length - 1 ? "file" : "dir"
      let node = level.find((n) => n.name === name && n.type === type)
      if (!node) {
        node = {name, path: prefix, type, children: []}
        level.push(node)
      }
      level = node.children
    })
  }
  sortTree(root)
  return root
}

function sortTree(nodes: TreeNode[]): void {
  nodes.sort((a, b) =>
    a.type !== b.type
      ? a.type === "dir"
        ? -1
        : 1
      : a.name.localeCompare(b.name, undefined, {sensitivity: "base"}),
  )
  for (const node of nodes) {
    sortTree(node.children)
  }
}

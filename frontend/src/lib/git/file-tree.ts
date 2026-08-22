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
// Linear find per level, O(files · siblings); swap in a Map keyed by name if
// a huge monorepo ever makes this lag.
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
        node = { name, path: prefix, type, children: [] }
        level.push(node)
      }
      level = node.children
    })
  }
  sortTree(root)
  return collapseChains(root)
}

// A changed-files tree is mostly corridor: src/main/java/br/com/acme holds
// nothing but the next directory, and a row per segment spends the panel's
// width on indentation until every file name is an ellipsis. A chain of
// directories with no branch in it collapses into one row — what GitHub's file
// tree and VS Code's compact folders show. The merged node keeps the deepest
// path as its id, so expand state stays unique per row.
function collapseChains(nodes: TreeNode[]): TreeNode[] {
  return nodes.map((node) => {
    let merged = node
    while (merged.children.length === 1 && merged.children[0].type === "dir") {
      const only = merged.children[0]
      merged = { ...only, name: `${merged.name}/${only.name}` }
    }
    return { ...merged, children: collapseChains(merged.children) }
  })
}

function sortTree(nodes: TreeNode[]): void {
  nodes.sort((a, b) =>
    a.type !== b.type
      ? a.type === "dir"
        ? -1
        : 1
      : a.name.localeCompare(b.name, undefined, { sensitivity: "base" }),
  )
  for (const node of nodes) {
    sortTree(node.children)
  }
}

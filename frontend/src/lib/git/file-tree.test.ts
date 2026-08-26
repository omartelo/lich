import { describe, expect, it } from "vitest"
import { buildTree, type TreeNode } from "./file-tree"

// names flattens a node list to "type:path" strings in order, so a test reads
// the whole shape and ordering in one assertion.
function names(nodes: TreeNode[]): string[] {
  return nodes.flatMap((n) => [`${n.type}:${n.path}`, ...names(n.children)])
}

describe("buildTree", () => {
  it("nests paths by their slash segments", () => {
    const tree = buildTree(["internal/rpc/rpc.go", "internal/x.go"])
    expect(names(tree)).toEqual([
      "dir:internal",
      "dir:internal/rpc",
      "file:internal/rpc/rpc.go",
      "file:internal/x.go",
    ])
  })

  it("collapses a chain of single-child directories into one row", () => {
    const tree = buildTree(["src/main/java/br/One.java", "src/main/java/br/Two.java"])
    expect(names(tree)).toEqual([
      "dir:src/main/java/br",
      "file:src/main/java/br/One.java",
      "file:src/main/java/br/Two.java",
    ])
    expect(tree[0]?.name).toBe("src/main/java/br")
  })

  it("stops collapsing where the tree branches", () => {
    const tree = buildTree(["a/b/c/one.go", "a/b/d/two.go"])
    expect(names(tree)).toEqual([
      "dir:a/b",
      "dir:a/b/c",
      "file:a/b/c/one.go",
      "dir:a/b/d",
      "file:a/b/d/two.go",
    ])
  })

  it("keeps a directory holding a single file as its own row", () => {
    expect(names(buildTree(["a/one.go"]))).toEqual(["dir:a", "file:a/one.go"])
  })

  it("merges siblings under a shared directory", () => {
    const tree = buildTree(["a/one.go", "a/two.go"])
    expect(names(tree)).toEqual(["dir:a", "file:a/one.go", "file:a/two.go"])
  })

  it("orders directories before files, each case-insensitively", () => {
    const tree = buildTree(["Zeta.md", "alpha.md", "src/b.ts", "src/A.ts"])
    expect(names(tree)).toEqual([
      "dir:src",
      "file:src/A.ts",
      "file:src/b.ts",
      "file:alpha.md",
      "file:Zeta.md",
    ])
  })

  it("returns an empty tree for no paths", () => {
    expect(buildTree([])).toEqual([])
  })

  it("ignores empty and malformed segments", () => {
    expect(buildTree([""])).toEqual([])
    expect(names(buildTree(["a//b"]))).toEqual(["dir:a", "file:a/b"])
  })
})

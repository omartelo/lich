import { describe, expect, it } from "vitest"
import { buildTree, treeFootnote, withDirStats, type TreeNode } from "./file-tree"

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

describe("treeFootnote", () => {
  it("a repository owes the reader nothing", () => {
    expect(treeFootnote(false, [])).toBe("")
  })

  it("names the directories the walk stepped over", () => {
    expect(treeFootnote(false, ["build", "node_modules"])).toBe("Hidden: build, node_modules.")
  })

  it("says a listing stopped short", () => {
    expect(treeFootnote(true, [])).toBe("This folder has more files than the tree can list.")
  })

  // Both, and the names first: they are the half a reader can act on.
  it("puts the names ahead of the cap", () => {
    expect(treeFootnote(true, ["vendor"])).toBe(
      "Hidden: vendor. This folder has more files than the tree can list.",
    )
  })
})

describe("withDirStats", () => {
  it("sums every changed file beneath a folder, at every level", () => {
    const tree = buildTree(["src/a.ts", "src/lib/b.ts", "src/lib/c.ts", "README.md"])
    const stats = withDirStats(
      tree,
      new Map([
        ["src/a.ts", { added: 3, deleted: 1 }],
        ["src/lib/b.ts", { added: 2, deleted: 0 }],
      ]),
    )
    expect(stats.get("src")).toEqual({ added: 5, deleted: 1 })
    expect(stats.get("src/lib")).toEqual({ added: 2, deleted: 0 })
    expect(stats.get("src/a.ts")).toEqual({ added: 3, deleted: 1 })
  })

  it("leaves a folder with no changed file out", () => {
    const tree = buildTree(["docs/guide.md", "src/a.ts"])
    const stats = withDirStats(tree, new Map([["src/a.ts", { added: 1, deleted: 0 }]]))
    expect(stats.has("docs")).toBe(false)
  })

  // The merged row is keyed by its deepest path, the id the row renders under.
  it("keys a collapsed chain by the row it renders as", () => {
    const tree = buildTree(["vc/src/main/A.jsp", "vc/src/main/B.jsp", "vc/build.gradle"])
    const stats = withDirStats(
      tree,
      new Map([
        ["vc/src/main/A.jsp", { added: 10, deleted: 0 }],
        ["vc/src/main/B.jsp", { added: 5, deleted: 2 }],
      ]),
    )
    expect(stats.get("vc/src/main")).toEqual({ added: 15, deleted: 2 })
    expect(stats.get("vc")).toEqual({ added: 15, deleted: 2 })
  })

  // A filtered tree sums what it shows: a changed file the filter hid does not
  // count toward the folder it would have sat in.
  it("counts only the files the tree holds", () => {
    const tree = buildTree(["src/a.ts"])
    const stats = withDirStats(
      tree,
      new Map([
        ["src/a.ts", { added: 1, deleted: 1 }],
        ["src/hidden.ts", { added: 50, deleted: 0 }],
      ]),
    )
    expect(stats.get("src")).toEqual({ added: 1, deleted: 1 })
  })
})

import {describe, expect, it} from "vitest"
import {
  buildFileDoc,
  discardTargets,
  formatLineRef,
  gutterNumber,
  newLineRange,
  parseDiff,
} from "./diff"

const modifiedTwoHunks = `diff --git a/src/app.ts b/src/app.ts
index 1111111..2222222 100644
--- a/src/app.ts
+++ b/src/app.ts
@@ -1,3 +1,4 @@
 const a = 1
-const b = 2
+const b = 3
+const c = 4
 export {}
@@ -10,2 +11,2 @@ function tail() {
-  return b
+  return c
   }`

const newFileDiff = `diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..b77b4eb
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+x
+y`

const deletedFileDiff = `diff --git a/gone.txt b/gone.txt
deleted file mode 100644
index b77b4eb..0000000
--- a/gone.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-x
-y`

const renamedDiff = `diff --git a/old-name.ts b/new-name.ts
similarity index 90%
rename from old-name.ts
rename to new-name.ts
index 1111111..2222222 100644
--- a/old-name.ts
+++ b/new-name.ts
@@ -5,2 +5,2 @@
-old line
+new line
 context`

const binaryDiff = `diff --git a/blob.bin b/blob.bin
new file mode 100644
index 0000000..ce542ef
Binary files /dev/null and b/blob.bin differ`

const noNewlineDiff = `diff --git a/f.txt b/f.txt
index 1111111..2222222 100644
--- a/f.txt
+++ b/f.txt
@@ -1 +1 @@
-old
\\ No newline at end of file
+new
\\ No newline at end of file`

describe("parseDiff", () => {
  it("parses a modified file with two hunks and resolves line numbers", () => {
    const files = parseDiff(modifiedTwoHunks)
    expect(files).toHaveLength(1)
    const file = files[0]
    expect(file.newPath).toBe("src/app.ts")
    expect(file.status).toBe("modified")
    expect(file.added).toBe(3)
    expect(file.deleted).toBe(2)
    expect(file.hunks).toHaveLength(2)

    const [first, second] = file.hunks
    expect(first.lines.map((l) => [l.kind, l.oldLine, l.newLine])).toEqual([
      ["context", 1, 1],
      ["del", 2, null],
      ["add", null, 2],
      ["add", null, 3],
      ["context", 3, 4],
    ])
    expect(second.newStart).toBe(11)
    expect(second.lines[1]).toMatchObject({kind: "add", newLine: 11})
  })

  it("parses an untracked file rendered by --no-index", () => {
    const file = parseDiff(newFileDiff)[0]
    expect(file.status).toBe("added")
    expect(file.newPath).toBe("new.txt")
    expect(file.added).toBe(2)
    expect(file.hunks[0].lines.map((l) => l.newLine)).toEqual([1, 2])
  })

  it("parses a deleted file", () => {
    const file = parseDiff(deletedFileDiff)[0]
    expect(file.status).toBe("deleted")
    expect(file.oldPath).toBe("gone.txt")
    expect(file.deleted).toBe(2)
  })

  it("parses a rename keeping both paths", () => {
    const file = parseDiff(renamedDiff)[0]
    expect(file.status).toBe("renamed")
    expect(file.oldPath).toBe("old-name.ts")
    expect(file.newPath).toBe("new-name.ts")
    expect(file.hunks[0].lines[1]).toMatchObject({kind: "add", newLine: 5})
  })

  it("flags binary files without hunks", () => {
    const file = parseDiff(binaryDiff)[0]
    expect(file.binary).toBe(true)
    expect(file.hunks).toHaveLength(0)
  })

  it("treats no-newline markers as meta lines", () => {
    const file = parseDiff(noNewlineDiff)[0]
    const kinds = file.hunks[0].lines.map((l) => l.kind)
    expect(kinds).toEqual(["del", "meta", "add", "meta"])
    expect(file.added).toBe(1)
    expect(file.deleted).toBe(1)
  })

  it("parses several files from one diff", () => {
    const files = parseDiff(`${modifiedTwoHunks}\n${newFileDiff}`)
    expect(files.map((f) => f.newPath)).toEqual(["src/app.ts", "new.txt"])
  })

  it("returns [] for empty input", () => {
    expect(parseDiff("")).toEqual([])
  })
})

describe("buildFileDoc", () => {
  it("strips diff prefixes and separates hunks with one blank line", () => {
    const doc = buildFileDoc(parseDiff(modifiedTwoHunks)[0])
    const docLines = doc.text.split("\n")
    expect(docLines).toHaveLength(doc.lineMeta.length)
    expect(docLines[0]).toBe("const a = 1")
    expect(doc.lineMeta[0]).toMatchObject({kind: "context", oldLine: 1, newLine: 1})
    expect(doc.lineMeta[1]).toMatchObject({
      kind: "del",
      oldLine: 2,
      text: "const b = 2",
    })
    expect(docLines[5]).toBe("")
    expect(doc.lineMeta[5].kind).toBe("meta")
    expect(doc.lineMeta[7]).toMatchObject({kind: "add", newLine: 11})
  })

  it("omits no-newline markers from the doc", () => {
    const doc = buildFileDoc(parseDiff(noNewlineDiff)[0])
    expect(doc.text.split("\n")).toEqual(["old", "new"])
    expect(doc.lineMeta.map((l) => l.kind)).toEqual(["del", "add"])
  })

  it("yields an empty doc for a binary file", () => {
    const doc = buildFileDoc(parseDiff(binaryDiff)[0])
    expect(doc.text).toBe("")
    expect(doc.lineMeta).toEqual([])
  })
})

describe("newLineRange", () => {
  const meta = buildFileDoc(parseDiff(modifiedTwoHunks)[0]).lineMeta
  // doc lines: 1 ctx(1), 2 del, 3 add(2), 4 add(3), 5 ctx(4),
  //            6 sep, 7 del, 8 add(11), 9 ctx(12)

  it("maps a pure-addition selection", () => {
    expect(newLineRange(meta, 3, 4)).toEqual({start: 2, end: 3})
  })

  it("collapses a single line to start === end", () => {
    expect(newLineRange(meta, 3, 3)).toEqual({start: 2, end: 2})
  })

  it("returns null for deleted/separator-only selections", () => {
    expect(newLineRange(meta, 2, 2)).toBeNull()
    expect(newLineRange(meta, 6, 6)).toBeNull()
    expect(newLineRange(meta, 6, 7)).toBeNull()
  })

  it("spans hunk boundaries with min/max of covered new lines", () => {
    expect(newLineRange(meta, 3, 8)).toEqual({start: 2, end: 11})
  })

  it("ignores separators inside the selection", () => {
    expect(newLineRange(meta, 5, 6)).toEqual({start: 4, end: 4})
  })
})

describe("formatLineRef", () => {
  it("collapses a single-line selection", () => {
    expect(formatLineRef({start: 19, end: 19})).toBe("19")
  })

  it("keeps a multi-line range", () => {
    expect(formatLineRef({start: 19, end: 24})).toBe("19-24")
  })
})

describe("discardTargets", () => {
  it("targets only the file itself for plain changes", () => {
    expect(discardTargets(parseDiff(modifiedTwoHunks)[0])).toEqual(["src/app.ts"])
    expect(discardTargets(parseDiff(newFileDiff)[0])).toEqual(["new.txt"])
  })

  it("targets both sides of a rename, new path first", () => {
    expect(discardTargets(parseDiff(renamedDiff)[0])).toEqual([
      "new-name.ts",
      "old-name.ts",
    ])
  })
})

describe("gutterNumber", () => {
  const meta = buildFileDoc(parseDiff(modifiedTwoHunks)[0]).lineMeta

  it("shows old numbers for deletions, new numbers otherwise, none for separators", () => {
    expect(gutterNumber(meta[0])).toBe("1") // context
    expect(gutterNumber(meta[1])).toBe("2") // del → old line
    expect(gutterNumber(meta[2])).toBe("2") // add → new line
    expect(gutterNumber(meta[5])).toBe("") // separator
  })
})

// @vitest-environment jsdom
//
// jsdom for the context-menu cases that need real markup: a fake closest()
// would have to reimplement the selectors it is meant to check.
import { describe, expect, it, vi } from "vitest"
import {
  installBrowserDefaults,
  isAppContextMenu,
  isBrowserChord,
  MOUSE_BACK_BUTTON,
  MOUSE_FORWARD_BUTTON,
} from "./browser-defaults"

const chord = (over: Partial<KeyboardEvent>) =>
  ({
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    altKey: false,
    key: "a",
    ...over,
  }) as KeyboardEvent

describe("isBrowserChord", () => {
  it("swallows the tab, window, file and page commands under either modifier", () => {
    for (const key of ["t", "w", "n", "p", "s", "o", "u", "d", "h", "j"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, key }))).toBe(true)
      expect(isBrowserChord(chord({ metaKey: true, key }))).toBe(true)
    }
  })

  it("swallows the shifted commands, including the browsing-data wipe", () => {
    for (const key of ["T", "W", "N", "M", "P", "O", "Q", "Delete"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key }))).toBe(true)
    }
  })

  it("leaves the devtools chords alone — Shift changes what the key means", () => {
    for (const key of ["i", "j", "c"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, shiftKey: true, key }))).toBe(false)
    }
  })

  it("leaves reload, editing chords and bare keys alone", () => {
    for (const key of ["r", "a", "c", "v", "x", "z", "f", "k"]) {
      expect(isBrowserChord(chord({ ctrlKey: true, key }))).toBe(false)
    }
    expect(isBrowserChord(chord({ key: "t" }))).toBe(false)
  })

  it("ignores AltGr, which arrives as Ctrl+Alt and types real characters", () => {
    expect(isBrowserChord(chord({ ctrlKey: true, altKey: true, key: "w" }))).toBe(false)
  })
})

describe("isAppContextMenu", () => {
  // Answers closest() the way the DOM would for the two selectors xterm's markup
  // can meet: .xterm, and .xterm without the class it sets while an app reads the mouse.
  const target = (inTerminal: boolean, mouseTracking = false) =>
    ({
      closest: (selector: string) => {
        const excluded = mouseTracking && selector.includes(":not(.enable-mouse-events)")
        return inTerminal && !excluded ? {} : null
      },
    }) as unknown as EventTarget

  it("claims the app's own chrome", () => {
    expect(isAppContextMenu(target(false))).toBe(true)
  })

  it("leaves a plain terminal's menu alone — that is where Copy and Paste live", () => {
    expect(isAppContextMenu(target(true))).toBe(false)
  })

  it("claims a terminal whose app reads the mouse, so only the app's menu shows", () => {
    expect(isAppContextMenu(target(true, true))).toBe(true)
  })

  it("claims a null target rather than letting the browser menu through", () => {
    expect(isAppContextMenu(null)).toBe(true)
  })
})

describe("isAppContextMenu on real markup", () => {
  const mount = (html: string) => {
    document.body.innerHTML = html
    return document.querySelector("[data-target]")
  }

  // docs/ceilings.md promises the window's editing items in a text field.
  it("leaves a text field's menu alone, so Cut, Copy and Paste are offered", () => {
    expect(isAppContextMenu(mount("<input data-target>"))).toBe(false)
    expect(isAppContextMenu(mount("<textarea data-target></textarea>"))).toBe(false)
    expect(
      isAppContextMenu(mount('<div contenteditable="true"><span data-target>x</span></div>')),
    ).toBe(false)
  })

  // A checkbox or a radio takes no text: the window would have nothing to put in
  // the menu, and a browser tab would offer its page menu instead.
  it("claims a checkbox and a radio, which are inputs without text", () => {
    expect(isAppContextMenu(mount('<input type="checkbox" data-target>'))).toBe(true)
    expect(isAppContextMenu(mount('<input type="radio" data-target>'))).toBe(true)
    expect(isAppContextMenu(mount('<input type="number" data-target>'))).toBe(false)
  })

  // A read-only CodeMirror view carries contenteditable="false": nothing to edit there.
  it("claims a contenteditable that is switched off", () => {
    expect(isAppContextMenu(mount('<div contenteditable="false" data-target>x</div>'))).toBe(true)
  })

  // xterm's helper textarea sits inside the terminal; the mouse-reading app
  // still owns the right button there.
  it("claims the textarea inside a terminal whose app reads the mouse", () => {
    const html = '<div class="xterm enable-mouse-events"><textarea data-target></textarea></div>'
    expect(isAppContextMenu(mount(html))).toBe(true)
  })
})

// The window is injected, so the wiring is checked without one: which phase each
// listener takes, and which events it is allowed to cancel. Both matter more than
// the matchers above — a keydown listener on the bubble phase reaches the browser
// too late, and Ctrl+W closes the window, leaving lich running with nothing on screen.
describe("installBrowserDefaults", () => {
  const install = (selectedText = "") => {
    const listeners = new Map<string, { handler: (event: unknown) => void; capture: unknown }>()
    const target = {
      addEventListener: (type: string, handler: (event: unknown) => void, capture?: unknown) => {
        listeners.set(type, { handler, capture })
      },
      getSelection: () => ({ toString: () => selectedText }),
    } as unknown as Window
    installBrowserDefaults(target)
    const fire = (type: string, event: Record<string, unknown>) => {
      const preventDefault = vi.fn()
      listeners.get(type)?.handler({ preventDefault, ...event })
      return preventDefault
    }
    return { listeners, fire }
  }

  it("takes the keydown in the capture phase, ahead of the browser", () => {
    expect(install().listeners.get("keydown")?.capture).toBe(true)
  })

  it("cancels a browser chord and leaves everything else through", () => {
    const { fire } = install()

    expect(
      fire("keydown", { ctrlKey: true, shiftKey: false, altKey: false, key: "w" }),
    ).toHaveBeenCalled()
    expect(
      fire("keydown", { ctrlKey: true, shiftKey: false, altKey: false, key: "r" }),
    ).not.toHaveBeenCalled()
  })

  // A plain terminal keeps Chromium's menu — that is where its Copy and Paste live.
  it("cancels the context menu everywhere but a plain terminal", () => {
    const { fire } = install()

    expect(fire("contextmenu", { target: { closest: () => null } })).toHaveBeenCalled()
    expect(fire("contextmenu", { target: { closest: () => ({}) } })).not.toHaveBeenCalled()
  })

  // Selected text copies from the window's menu, wherever it sits: a path, an
  // error, a PR description.
  it("leaves the menu alone while text is selected, so Copy is offered", () => {
    const { fire } = install("/home/you/src/lich")

    expect(fire("contextmenu", { target: { closest: () => null } })).not.toHaveBeenCalled()
  })

  // Bubble phase, so a drop zone of our own runs first; unconditional, because
  // whatever is left has nowhere to land but over the app itself.
  it("cancels every unclaimed drag and drop, in the bubble phase", () => {
    const { listeners, fire } = install()

    expect(listeners.get("drop")?.capture).toBeUndefined()
    expect(listeners.get("dragover")?.capture).toBeUndefined()
    expect(fire("drop", {})).toHaveBeenCalled()
    expect(fire("dragover", {})).toHaveBeenCalled()
  })

  // The cursor is the only answer a drag gets before it lands: anything but
  // "none" tells the user the window will take the file.
  it("answers an unclaimed dragover with no drop allowed", () => {
    const { fire } = install()
    const dataTransfer = { dropEffect: "copy" }

    fire("dragover", { defaultPrevented: false, dataTransfer })

    expect(dataTransfer.dropEffect).toBe("none")
  })

  // A terminal's drop zone runs first and claims the drag with preventDefault;
  // its "copy" must survive the window listener.
  it("leaves a claimed dragover's drop effect alone", () => {
    const { fire } = install()
    const dataTransfer = { dropEffect: "copy" }

    fire("dragover", { defaultPrevented: true, dataTransfer })

    expect(dataTransfer.dropEffect).toBe("copy")
  })

  // Capture phase, so a terminal that stops the event still has its default
  // cancelled; preventDefault only, so the PTY still reads the button.
  it("cancels the mouse back and forward buttons, in the capture phase", () => {
    const { listeners, fire } = install()

    expect(listeners.get("mouseup")?.capture).toBe(true)
    expect(fire("mouseup", { button: MOUSE_BACK_BUTTON })).toHaveBeenCalled()
    expect(fire("mouseup", { button: MOUSE_FORWARD_BUTTON })).toHaveBeenCalled()
    for (const button of [0, 1, 2]) {
      expect(fire("mouseup", { button })).not.toHaveBeenCalled()
    }
  })
})

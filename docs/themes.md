# Theme JSON

lich ships bundled `light` and `dark` themes and accepts user-imported JSON themes.
The Appearance settings can save a valid dark starter template that uses only hex
colors; it is `themes/template.json`, embedded in the binary, and it names every
supported color.
Imported themes are stored under the user config directory:

- Production: `<config-dir>/lich/themes/<id>.json`
- Dev mode (`LICH_DEV`): `<config-dir>/lich/themes-dev/<id>.json`

The file name is managed by lich on import. A stored custom theme whose file name
does not match `<id>.json` is ignored.

## Import Shape

```json
{
  "id": "tokyo-night",
  "name": "Tokyo Night",
  "scheme": "dark",
  "app": {
    "background": "oklch(0.18 0.05 276)",
    "foreground": "oklch(0.92 0.03 270)",
    "card": "oklch(0.22 0.05 276)",
    "card-foreground": "oklch(0.92 0.03 270)",
    "popover": "oklch(0.22 0.05 276)",
    "popover-foreground": "oklch(0.92 0.03 270)",
    "primary": "oklch(0.75 0.13 255)",
    "primary-foreground": "oklch(0.14 0.04 276)",
    "secondary": "oklch(0.27 0.05 276)",
    "secondary-foreground": "oklch(0.88 0.04 270)",
    "muted": "oklch(0.25 0.04 276)",
    "muted-foreground": "oklch(0.65 0.05 270)",
    "accent": "oklch(0.32 0.07 276)",
    "accent-foreground": "oklch(0.92 0.03 270)",
    "destructive": "oklch(0.7 0.18 25)",
    "border": "oklch(0.95 0.02 270 / 12%)",
    "input": "oklch(0.95 0.02 270 / 16%)",
    "ring": "oklch(0.75 0.13 255)",
    "chart-1": "oklch(0.75 0.13 255)",
    "chart-2": "oklch(0.78 0.13 150)",
    "chart-3": "oklch(0.82 0.13 85)",
    "chart-4": "oklch(0.73 0.15 320)",
    "chart-5": "oklch(0.76 0.14 25)",
    "sidebar": "oklch(0.16 0.04 276)",
    "sidebar-foreground": "oklch(0.92 0.03 270)",
    "sidebar-primary": "oklch(0.75 0.13 255)",
    "sidebar-primary-foreground": "oklch(0.14 0.04 276)",
    "sidebar-accent": "oklch(0.27 0.05 276)",
    "sidebar-accent-foreground": "oklch(0.92 0.03 270)",
    "sidebar-border": "oklch(0.95 0.02 270 / 12%)",
    "sidebar-ring": "oklch(0.75 0.13 255)"
  },
  "terminal": {
    "background": "#1a1b26",
    "foreground": "#c0caf5",
    "cursor": "#c0caf5",
    "selectionBackground": "#33467c"
  }
}
```

`origin` is optional in imported JSON. The backend overwrites it with `custom`
before saving.

## Validation Rules

- `id` is required, must match `^[a-z0-9][a-z0-9._-]{0,63}$`, and cannot be
  `light`, `dark`, `system`, or `match`. Windows device names (`con`, `prn`,
  `aux`, `nul`, `com1`-`com9`, `lpt1`-`lpt9`) are rejected too: a theme id names
  a file, and those still address a device even with an extension.
- `name` is required, cannot be blank, and cannot exceed 128 characters.
- `scheme` must be `light` or `dark`.
- `app` is required and must include every token shown in the example.
- `terminal.background` and `terminal.foreground` are required.
- Optional terminal tokens are: `cursor`, `cursorAccent`, `selectionBackground`,
  `selectionForeground`, `black`, `red`, `green`, `yellow`, `blue`, `magenta`,
  `cyan`, `white`, `brightBlack`, `brightRed`, `brightGreen`, `brightYellow`,
  `brightBlue`, `brightMagenta`, `brightCyan`, `brightWhite`.
- Unknown app or terminal tokens are rejected.
- No color value can be blank or exceed 128 characters.

Each color block takes what its consumer actually parses:

- **`app`** values become CSS custom properties, so they accept hex
  (`#rgb` through `#rrggbbaa`), a CSS color name, or a color function —
  `rgb()`, `rgba()`, `hsl()`, `hsla()`, `oklch()`, `oklab()`, `lab()`, `lch()`,
  `color()`. Anything that could name a resource or defer a value — `url()`,
  `image-set()`, `var()`, `attr()`, `color-mix()` — is rejected.
- **`terminal`** values must be hex. xterm parses hex directly; every other form
  goes through a round-trip that fails on translucency and is swallowed into a
  silent fallback, which would apply a theme only halfway with no error shown.

Import uses lich's native file picker. Re-importing a theme whose `id` already
exists asks for confirmation before replacing the stored file; cancelling the
confirmation leaves the existing theme unchanged.

The template download uses the native save dialog, so the file lands where you
choose it; the dialog confirms replacing an existing file.

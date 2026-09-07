{
    "##": [
        "Scoop manifest for lich. The release workflow renders this template with the tag's version and the",
        "sha256 of the two Windows assets from its checksums.txt, and ships the result as lich.json beside",
        "them, so the install URL is the latest release's copy and `scoop update lich` re-reads it:",
        "  scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json",
        "There is no bucket repository, and nothing on a branch to bump: a manifest that pins one release",
        "and lives on main is stale the moment the next tag lands, which is how 0.46.0 shipped installing",
        "0.45.0.",
        "Two assets, because the window is a separate binary: lich.exe alone is a backend that has to fall back",
        "to a system browser. internal/chromium/shell.go looks for shell/lich-shell.exe beside the executable",
        "first, and lich-<tag>-windows-amd64-shell.zip already carries shell/ at its root, so Scoop's default",
        "extraction lands it exactly there. That is the layout build/windows/lich.iss installs too."
    ],
    "version": "@VERSION@",
    "description": "A personal harness for AI-assisted development",
    "homepage": "https://github.com/omartelo/lich",
    "license": "AGPL-3.0-only",
    "url": [
        "https://github.com/omartelo/lich/releases/download/v@VERSION@/lich-v@VERSION@-windows-amd64.exe#/lich.exe",
        "https://github.com/omartelo/lich/releases/download/v@VERSION@/lich-v@VERSION@-windows-amd64-shell.zip"
    ],
    "hash": [
        "@SHA_EXE@",
        "@SHA_SHELL@"
    ],
    "bin": "lich.exe",
    "shortcuts": [
        [
            "lich.exe",
            "lich"
        ]
    ],
    "notes": [
        "lich keeps its workspace in %APPDATA%\\lich, so it survives an uninstall; `scoop uninstall lich` leaves it.",
        "The binaries are unsigned, so SmartScreen warns on first launch. Update with `scoop update lich`."
    ]
}

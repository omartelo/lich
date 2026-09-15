{
    "##": [
        "Scoop manifest for lich. The release workflow renders this template with the tag's version and the",
        "sha256 of the Windows zip from its checksums.txt, and ships the result as lich.json beside",
        "them, so the install URL is the latest release's copy and `scoop update lich` re-reads it:",
        "  scoop install https://github.com/omartelo/lich/releases/latest/download/lich.json",
        "There is no bucket repository, and nothing on a branch to bump: a manifest that pins one release",
        "and lives on main is stale the moment the next tag lands, which is how 0.46.0 shipped installing",
        "0.45.0.",
        "One asset: lich-<tag>-windows-amd64.zip is the portable package, lich.exe with the window beside it as",
        "shell\\ at the zip's root, so Scoop's default extraction lands both in the app directory. That is the",
        "layout build/windows/lich.iss installs too."
    ],
    "version": "@VERSION@",
    "description": "A terminal-first ADE for the coding agents you already use",
    "homepage": "https://github.com/omartelo/lich",
    "license": "AGPL-3.0-only",
    "url": "https://github.com/omartelo/lich/releases/download/v@VERSION@/lich-v@VERSION@-windows-amd64.zip",
    "hash": "@SHA_ZIP@",
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

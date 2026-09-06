#!/usr/bin/env python3
"""Renders lich's app icon out of the bare mark.

The mark alone (appicon-mark.png, white on transparent) vanishes on a light
taskbar, dock or launcher, and no desktop swaps an app icon with its theme;
the plate is what makes it readable on either. Outputs: appicon.png (1024,
what the packages, the macOS bundle and the shell's window take),
appicon-256.png (the window icon the shell compiles in) and
windows/lich.ico (the executables' resource; `task icons:windows` after).
Needs Pillow: python3 -m pip install pillow
"""
from pathlib import Path

from PIL import Image, ImageDraw

HERE = Path(__file__).parent
SIZE = 1024
# The app's dark ground (zinc-900), so the plate reads as lich's own surface.
PLATE = (24, 24, 27, 255)
# The corner radius desktops draw their own icons with, about a fifth of the side.
RADIUS = int(SIZE * 0.22)
# The mark fills two thirds of the plate's height; the rest is breathing room.
MARK_HEIGHT = int(SIZE * 0.64)

mark = Image.open(HERE / "appicon-mark.png").convert("RGBA")
mark = mark.crop(mark.getbbox())
mark = mark.resize((round(mark.width * MARK_HEIGHT / mark.height), MARK_HEIGHT), Image.LANCZOS)

icon = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
ImageDraw.Draw(icon).rounded_rectangle((0, 0, SIZE - 1, SIZE - 1), radius=RADIUS, fill=PLATE)
icon.alpha_composite(mark, ((SIZE - mark.width) // 2, (SIZE - mark.height) // 2))

icon.save(HERE / "appicon.png", optimize=True)
icon.resize((256, 256), Image.LANCZOS).save(HERE / "appicon-256.png", optimize=True)
icon.resize((256, 256), Image.LANCZOS).save(
    HERE / "windows" / "lich.ico",
    sizes=[(256, 256), (128, 128), (64, 64), (48, 48), (32, 32), (16, 16)],
)

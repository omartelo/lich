<?xml version="1.0" encoding="UTF-8"?>
<!--
  Rendered by build/darwin/bundle.sh for each of the five helper apps CEF's
  subprocesses run as (@NAME@ -> "Lich Helper", "Lich Helper (Renderer)", ...;
  @ROLE@ -> "", ".renderer", ...). Chromium derives the per-role app from the
  base one, and inside a bundle launches the renderer through that derivation
  alone. LSUIElement: a subprocess owns no window and gets no Dock tile.
-->
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>@NAME@</string>
  <key>CFBundleIdentifier</key>
  <string>com.github.omartelo.lich.helper@ROLE@</string>
  <key>CFBundleName</key>
  <string>@NAME@</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>@VERSION@</string>
  <key>CFBundleVersion</key>
  <string>@VERSION@</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>

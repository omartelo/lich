<?xml version="1.0" encoding="UTF-8"?>
<!--
  Rendered by build/darwin/bundle.sh (@VERSION@ -> the release version).

  lich is a pure-Go binary that never touches AppKit, so it never registers
  with the window server: macOS shows a Dock tile while LaunchServices starts
  it and drops that tile once the window belongs to the browser instead.
  Measured on a Mac, LSUIElement does not suppress that launch tile — it is
  here because an app that owns no window of its own has no business in the
  Dock or in Cmd-Tab once it is running. The icon that matters is the one in
  /Applications, Launchpad and Spotlight.
-->
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>
  <string>lich</string>
  <key>CFBundleIdentifier</key>
  <string>com.github.omartelo.lich</string>
  <key>CFBundleName</key>
  <string>lich</string>
  <key>CFBundleDisplayName</key>
  <string>lich</string>
  <key>CFBundleIconFile</key>
  <string>lich</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>@VERSION@</string>
  <key>CFBundleVersion</key>
  <string>@VERSION@</string>
  <key>LSMinimumSystemVersion</key>
  <string>11.0</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>

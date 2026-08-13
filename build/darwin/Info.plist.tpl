<?xml version="1.0" encoding="UTF-8"?>
<!--
  Rendered by build/darwin/bundle.sh (@VERSION@ -> the release version).

  LSUIElement is the measured answer, not a preference: lich is a pure-Go
  binary that never touches AppKit, so macOS shows its Dock tile only while
  LaunchServices is starting it and drops the tile as soon as the process
  fails to register with the window server. Without this key the user watches
  an icon appear and vanish; with it the app lives in /Applications, Launchpad
  and Spotlight, and the Dock shows only the browser that owns the window.
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

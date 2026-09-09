<?xml version="1.0" encoding="UTF-8"?>
<!--
  Rendered by build/darwin/bundle.sh (@VERSION@ -> the release version).

  Two executables share this plist: lich, the pure-Go binary LaunchServices
  starts, which never touches AppKit, and lich-shell, the window, which does.
  macOS reads a process's bundle off its executable's path, so the window in
  Contents/MacOS is Lich.app to the Dock, to Cmd-Tab and to the menu bar: its
  icon is CFBundleIconFile and its name CFBundleName. The Intel build ships no
  window and opens as a browser tab, so its Dock tile is the browser's;
  LSUIElement made no measured difference to that on a Mac and is not here.
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
  <string>13.0</string>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>

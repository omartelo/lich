# The Windows window (`shell\`)

The same crate as [the Linux window](../../linux/shell/README.md), built on
Windows with MSVC, and laid out the way CEF expects there: flat, the runtime
beside `lich-shell.exe`, no `cef\` subdirectory. `task build:shell` runs
cargo and `assemble.sh`; `task package:windows` puts the result inside the
installer as `shell\` next to `lich.exe`.

What Windows has no WM_CLASS for is done two other ways:

- `lich-shell.exe` carries lich's icon and version block as resources, and
  the executable manifest CEF asks for (`lich-shell.exe.manifest`), compiled
  in by `shell/build.rs`: Explorer, the Start Menu and Alt-Tab draw the icon
  from there.
- The process claims the AppUserModelID `omartelo.lich` (`omartelo.lichdev`
  for `task dev`), and the Start Menu shortcut declares the same one
  (`lich.iss`), so the taskbar groups the running window under the pinned
  icon rather than under a second button.

The window's icon in the title bar and on the taskbar button is the PNG the
shell hands CEF (`App::window_icon`, `build/appicon.png`); the executable's
own icon resource covers Explorer, Alt-Tab and the moment before the window
exists. The C runtime is linked statically: `vcruntime140.dll` is the Visual
C++ redistributable's, not Windows's, and a clean machine has none.

The sandbox stays off, as kurogane runs it on every platform, so the binary
is a plain exe and not CEF's `bootstrap.exe` loading a DLL. Building needs
Ninja on top of the runner's CMake and MSVC, and the CMake of CEF's wrapper
only finds MSVC from inside a Visual Studio developer prompt.

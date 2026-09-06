//! lich's window: an embedded Chromium (CEF, through kurogane) that the Go
//! backend launches exactly the way it launches a system browser. The argv is
//! the contract of `internal/chromium.Args`: `--app=<url>` is the page,
//! `--class=<name>` names the window for the window manager,
//! `--user-data-dir=<dir>` is where the profile lives, and every other
//! `--switch[=value]` is a Chromium switch to honour (`lich -- --ozone-platform=wayland`).
use kurogane::App;

/// The title the window carries. The page title is never used for it.
const TITLE: &str = "lich";

/// The icon the window carries: title bar, taskbar button and app switcher
/// on Windows, _NET_WM_ICON under X11 (Wayland draws the desktop entry's
/// instead). A 256 px copy of build/appicon.png (build/appicon.py renders
/// both): X11 stores the icon as raw pixels on the window, 8 MB at 1024,
/// 256 KB at this size.
const ICON: &[u8] = include_bytes!("../../build/appicon-256.png");

/// What the window's AppUserModelID starts with; the class follows. The Start
/// Menu shortcut in build/windows/lich.iss spells the same id out.
#[cfg(windows)]
const AUMID_PREFIX: &str = "omartelo.";

#[derive(Debug, Default, PartialEq)]
struct Launch {
    url: Option<String>,
    class: Option<String>,
    profile_dir: Option<String>,
    switches: Vec<(String, Option<String>)>,
}

fn parse<I: IntoIterator<Item = String>>(args: I) -> Launch {
    let mut launch = Launch::default();
    for arg in args {
        let Some(switch) = arg.strip_prefix("--") else {
            continue;
        };
        let (name, value) = match switch.split_once('=') {
            Some((name, value)) => (name, Some(value.to_owned())),
            None => (switch, None),
        };
        match name {
            // A bare `--` is a separator, not a switch.
            "" => {}
            "app" => launch.url = value,
            "class" => launch.class = value,
            "user-data-dir" => launch.profile_dir = value,
            // Feature lists stay in argv, where CEF reads them and unions them
            // with the features it disables itself. Pushed through kurogane
            // they would land as a plain switch write after that union and
            // replace it — with `--disable-features=Translate` alone, the
            // window came up with Chrome's Glic actor UI enabled and
            // segfaulted attaching its first tab (measured, CEF 150).
            "disable-features" | "enable-features" => {}
            _ => launch.switches.push((name.to_owned(), value)),
        }
    }
    launch
}

/// The display the window opens on. Chromium's own default is the hint `auto`,
/// Wayland whenever WAYLAND_DISPLAY is set, and the system browser lich
/// launched before the window followed it. kurogane instead forces X11 on
/// NVIDIA, and an XWayland window under a Wayland file manager loses every
/// file drop to the compositor's DnD bridge (Hyprland, measured; its issue
/// #7800). Set before the user's switches, so `lich -- --ozone-platform=x11`
/// still wins.
fn display_switch(wayland_display: Option<&str>) -> Option<(&'static str, &'static str)> {
    wayland_display
        .filter(|display| !display.is_empty())
        .map(|_| ("ozone-platform", "wayland"))
}

fn main() {
    // CEF re-executes this binary for the renderer, GPU and utility roles with
    // an argv of its own. Those roles exit inside run_or_exit before any window
    // exists, so a missing --app= is a subprocess, not an error.
    // kurogane takes the runtime from CEF_PATH before the one beside the
    // executable, a developer's override that a CEF developer's shell would
    // carry into lich (the CI runner's did: "invalid CEF runtime at .cef").
    // The window lich ships is the only runtime it runs on.
    // SAFETY: no other thread exists yet, so nothing reads the environment
    // concurrently.
    unsafe { std::env::remove_var("CEF_PATH") };
    let launch = parse(std::env::args().skip(1));
    #[cfg(windows)]
    if let Some(class) = &launch.class {
        claim_taskbar_identity(class);
    }
    let mut app = App::url(launch.url.unwrap_or_else(|| "about:blank".into()))
        // Only reached without --user-data-dir, which lich always passes: a
        // shell launched by hand gets a profile under its own name rather than
        // kurogane's default.
        .profile_id("lich")
        .window_title(TITLE)
        .window_icon(ICON)
        // CEF's Chrome runtime loads the extensions a distribution installs
        // system-wide (plasma-browser-integration on KDE). A window that is
        // not a browser has no use for them.
        .chromium_flag("disable-extensions");
    // Chromium keeps the key for its cookie store in the Keychain, granted to
    // one code identity; an ad-hoc signed binary gets a new one every build,
    // and every start would ask the user for the Keychain again. The only
    // cookie this window holds is lich's own localhost session.
    #[cfg(target_os = "macos")]
    {
        app = app.credential_storage(kurogane::CredentialStorage::Basic);
    }
    if let Some(class) = launch.class {
        app = app.window_class(class);
    }
    // The profile goes where lich keeps it for any browser. Explicitly, not
    // through the XDG cache directory kurogane would derive it from: Chromium
    // rewrites XDG_CACHE_HOME for itself from --user-data-dir, and a profile
    // located through that variable moves with it.
    if let Some(dir) = launch.profile_dir {
        app = app.cache_dir(dir);
    }
    let wayland = std::env::var("WAYLAND_DISPLAY").ok();
    if let Some((name, value)) = display_switch(wayland.as_deref()) {
        app = app.chromium_flag_with_value(name, value);
    }
    // The user's switches go through kurogane rather than staying in argv, so
    // they land after its own policy and win: --ozone-platform=x11 beats the
    // Wayland chosen above.
    for (name, value) in launch.switches {
        app = match value {
            Some(value) => app.chromium_flag_with_value(name, value),
            None => app.chromium_flag(name),
        };
    }
    app.run_or_exit();
}

/// The AppUserModelID is Windows's WM_CLASS: the taskbar groups a process's
/// windows under it and draws the icon of the Start Menu shortcut carrying the
/// same id (lich.iss declares AUMID_PREFIX + "lich"), so the running window
/// and the pinned one are one button. The dev shell's own class keeps it off
/// the daily driver's. Only the browser process has a class on its argv, and
/// only it is called here; CEF's subprocesses never own a window. Best
/// effort: without it the button stands alone under the executable's icon.
#[cfg(windows)]
fn claim_taskbar_identity(class: &str) {
    use windows_sys::Win32::UI::Shell::SetCurrentProcessExplicitAppUserModelID;
    let id: Vec<u16> = format!("{AUMID_PREFIX}{class}\0").encode_utf16().collect();
    // SAFETY: id is NUL-terminated and outlives the call, which copies it.
    let _ = unsafe { SetCurrentProcessExplicitAppUserModelID(id.as_ptr()) };
}

#[cfg(test)]
mod tests {
    use super::*;

    fn args(list: &[&str]) -> Vec<String> {
        list.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn splits_lich_switches_from_chromium_switches() {
        let launch = parse(args(&[
            "--app=http://127.0.0.1:47821/?token=x",
            "--user-data-dir=/home/u/.config/lich/chromium-profile",
            "--profile-directory=Default",
            "--class=lich",
            "--no-first-run",
            "--disable-features=Translate",
        ]));
        assert_eq!(
            launch,
            Launch {
                url: Some("http://127.0.0.1:47821/?token=x".into()),
                class: Some("lich".into()),
                profile_dir: Some("/home/u/.config/lich/chromium-profile".into()),
                switches: vec![
                    ("profile-directory".into(), Some("Default".into())),
                    ("no-first-run".into(), None),
                ],
            }
        );
    }

    #[test]
    fn leaves_feature_lists_to_cef() {
        let launch = parse(args(&[
            "--disable-features=Translate",
            "--enable-features=Vulkan",
            "--ozone-platform=wayland",
        ]));
        assert_eq!(
            launch.switches,
            vec![("ozone-platform".into(), Some("wayland".into()))]
        );
    }

    #[test]
    fn keeps_the_first_equals_inside_a_value() {
        let launch = parse(args(&["--app=http://h/?a=1&b=2"]));
        assert_eq!(launch.url.as_deref(), Some("http://h/?a=1&b=2"));
    }

    #[test]
    fn opens_native_wayland_when_a_wayland_display_exists() {
        assert_eq!(
            display_switch(Some("wayland-1")),
            Some(("ozone-platform", "wayland"))
        );
        assert_eq!(display_switch(Some("")), None);
        assert_eq!(display_switch(None), None);
    }

    #[test]
    fn ignores_what_is_not_a_switch() {
        let launch = parse(args(&["run", "-x", "--", "http://h/"]));
        assert_eq!(launch, Launch::default());
    }
}

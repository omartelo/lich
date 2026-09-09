//! lich's window: an embedded Chromium (CEF, through kurogane) that the Go
//! backend launches. The argv is the contract of `internal/chromium.Args`:
//! `--url=<url>` is the page, `--class=<name>` names the window for the window
//! manager, `--user-data-dir=<dir>` is where the profile lives, and every other
//! `--switch[=value]` is a Chromium switch to honour (`lich -- --ozone-platform=wayland`).
// Rust defaults to the console subsystem and lich.exe is built for the GUI one
// (-H=windowsgui), so there is no console to inherit: Windows allocated a fresh
// one per process, for the window and for every CEF subprocess. Closing it sent
// CTRL_CLOSE_EVENT to the group and killed the window with 0xc000013a inside the
// startup grace, which lich reads as a window that could not open.
#![windows_subsystem = "windows"]
use cef::rc::Rc as _;
use cef::sys::cef_event_flags_t;
use cef::{
    Browser, ImplKeyboardHandler, KeyEvent, KeyboardHandler, WrapKeyboardHandler,
    wrap_keyboard_handler,
};
use kurogane::{App, ClientAppBrowserDelegate};
use std::io::Read;
use std::os::raw::c_int;

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
    /// lich's own switch (internal/chromium/launch): end when stdin does.
    exit_on_stdin_eof: bool,
    switches: Vec<(String, Option<String>)>,
}

impl Launch {
    /// CEF re-executes the binary for its renderer, GPU and utility roles,
    /// each with a --type of its own; the browser process has none.
    #[cfg(target_os = "linux")]
    fn is_subprocess(&self) -> bool {
        self.switches.iter().any(|(name, _)| name == "type")
    }
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
            "url" => launch.url = value,
            "class" => launch.class = value,
            "user-data-dir" => launch.profile_dir = value,
            "exit-on-stdin-eof" => launch.exit_on_stdin_eof = true,
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
/// Wayland whenever WAYLAND_DISPLAY is set. kurogane instead forces X11 on
/// NVIDIA, and an XWayland window under a Wayland file manager loses every
/// file drop to the compositor's DnD bridge (Hyprland, measured; its issue
/// #7800). Set before the user's switches, so `lich -- --ozone-platform=x11`
/// still wins.
fn display_switch(wayland_display: Option<&str>) -> Option<(&'static str, &'static str)> {
    wayland_display
        .filter(|display| !display.is_empty())
        .map(|_| ("ozone-platform", "wayland"))
}

/// CEF's native event for a key press, never read: what the window decides, it
/// decides from the CefKeyEvent. Its type is the platform's own, so the
/// signature CEF asks for differs on each.
#[cfg(target_os = "linux")]
type OsEvent<'a> = Option<&'a mut cef::sys::XEvent>;
#[cfg(target_os = "windows")]
type OsEvent<'a> = Option<&'a mut cef::sys::MSG>;
#[cfg(target_os = "macos")]
type OsEvent<'a> = *mut u8;

/// Whether the page is given first refusal on a chord: every one carrying the
/// primary modifier: Ctrl on Linux and Windows, Cmd on macOS.
// The cast is required on Windows, where the flag's C int is signed, and
// redundant on Linux and macOS, where it is not: the lint has to go, because
// neither spelling alone builds on all three.
#[allow(clippy::unnecessary_cast)]
fn page_first(modifiers: u32) -> bool {
    const PRIMARY: u32 = cef_event_flags_t::EVENTFLAG_CONTROL_DOWN.0 as u32
        | cef_event_flags_t::EVENTFLAG_COMMAND_DOWN.0 as u32;
    modifiers & PRIMARY != 0
}

// Chromium reserves its tab accelerators: Ctrl+T, Ctrl+W, Ctrl+Shift+T and the
// rest run in the browser *before* the key is sent to the renderer, so a page
// binding one never sees it: lich's Ctrl+Shift+T reopened a closed tab rather
// than starting a session, in a window that has no tabs to reopen into. Marking a
// chord a keyboard shortcut is how CEF inverts that order: the renderer gets the
// key first, and Chromium runs the accelerator only if the page let it through
// (measured on CEF 150; a vetoed CefCommandHandler command cannot do it, the
// reserved ones never reach it).
wrap_keyboard_handler! {
    struct PageFirstKeys;

    impl KeyboardHandler {
        fn on_pre_key_event(
            &self,
            _browser: Option<&mut Browser>,
            event: Option<&KeyEvent>,
            _os_event: OsEvent<'_>,
            is_keyboard_shortcut: Option<&mut c_int>,
        ) -> c_int {
            if let (Some(event), Some(shortcut)) = (event, is_keyboard_shortcut)
                && page_first(event.modifiers)
            {
                *shortcut = 1;
            }
            0
        }
    }
}

/// CEF asks for a handler on every key event, so the window holds one and hands
/// out clones rather than building it in the getter.
struct Window {
    keys: KeyboardHandler,
}

impl ClientAppBrowserDelegate for Window {
    fn keyboard_handler(&self) -> Option<KeyboardHandler> {
        Some(self.keys.clone())
    }
}

fn main() {
    // CEF re-executes this binary for the renderer, GPU and utility roles with
    // an argv of its own. Those roles exit inside run_or_exit before any window
    // exists, so a missing --url= is a subprocess, not an error.
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
    // Only the browser process decides; CEF's subprocesses carry --type and
    // inherit the switch from it.
    #[cfg(target_os = "linux")]
    let unsandboxed = !launch.is_subprocess() && !sandbox_available();
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
        .chromium_flag("disable-extensions")
        .delegate(Window {
            keys: PageFirstKeys::new(),
        });
    // Chromium keeps the key for its cookie store in the Keychain, granted to
    // one code identity; an ad-hoc signed binary gets a new one every build,
    // and every start would ask the user for the Keychain again. The only
    // cookie this window holds is lich's own localhost session.
    #[cfg(target_os = "macos")]
    {
        app = app.credential_storage(kurogane::CredentialStorage::Basic);
    }
    // Chromium presents its frames through DirectComposition on Windows, and
    // an Intel UHD driver that cannot (31.0.101.2141, measured) leaves every
    // frame rendered and never shown: five live processes, the page loaded on
    // CDP, a white window. The classic swap chain costs the video overlays a
    // window with no video has no use for; --use-angle=gl and
    // --disable-gpu-compositing also clear it, but each swaps a whole backend
    // out, and the terminal's WebGL renderer rides on this one.
    #[cfg(windows)]
    {
        app = app.chromium_flag("disable-direct-composition");
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
    #[cfg(target_os = "linux")]
    if unsandboxed {
        eprintln!(
            "lich-shell: this machine cannot confine Chromium's subprocesses; opening the window unsandboxed"
        );
        app = app.chromium_flag("no-sandbox");
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
    // Last, after the fork in sandbox_available: that one must be the only
    // thread. Reached only from lich, whose end is what ends this window; a
    // shell launched by hand keeps whatever stdin it was given.
    if launch.exit_on_stdin_eof {
        std::thread::spawn(|| {
            wait_for_eof(std::io::stdin());
            std::process::exit(0);
        });
    }
    app.run_or_exit();
}

/// Blocks until stdin ends. lich holds the write end of the pipe it hands the
/// window for as long as it lives and never writes to it, so EOF here is lich
/// gone — killed, crashed, out of memory — and the window it opened goes with
/// it. Left running, the window was the orphan the next launch's window was
/// forwarded to by CEF's process singleton; that duplicate's exit read as the
/// window failing to open, and lich opened a system browser beside it.
fn wait_for_eof(mut stdin: impl Read) {
    let mut byte = [0u8; 1];
    while matches!(stdin.read(&mut byte), Ok(1..)) {}
}

/// Whether Chromium can confine its subprocesses here, decided the way
/// Chromium decides it at its zygote (content/browser/zygote_host): never as
/// root; otherwise a user namespace, or the setuid helper beside the executable.
/// With neither the browser process aborts at startup, and Chromium's own
/// advice is --no-sandbox; Ubuntu denies unprivileged user namespaces through
/// AppArmor, and the packages do not ship the helper setuid, so that is every
/// Ubuntu desktop. The "stability and security will suffer" bar Chrome draws
/// over the switch is then the truth about that machine.
#[cfg(target_os = "linux")]
fn sandbox_available() -> bool {
    // SAFETY: geteuid has no preconditions.
    if unsafe { libc::geteuid() } == 0 {
        return false;
    }
    if can_create_user_namespace() {
        return true;
    }
    std::env::current_exe()
        .ok()
        .and_then(|exe| exe.with_file_name("chrome-sandbox").metadata().ok())
        .is_some_and(|meta| {
            use std::os::unix::fs::MetadataExt;
            helper_usable(meta.uid(), meta.mode())
        })
}

/// What Chromium requires of the setuid helper: owned by root, setuid, and
/// executable by everyone (sandbox/linux/suid/client/setuid_sandbox_host.cc).
#[cfg(target_os = "linux")]
fn helper_usable(uid: u32, mode: u32) -> bool {
    uid == 0 && mode & libc::S_ISUID != 0 && mode & libc::S_IXOTH != 0
}

/// Chromium's own probe (sandbox::Credentials::CanCreateProcessInNewUserNS):
/// a child tries to enter a new user namespace, and its exit status is the
/// answer. Asked from a process, not read from a sysctl, because what denies
/// the namespace on Ubuntu is an AppArmor policy on unconfined processes, which
/// is what lich-shell is and a probe binary with a profile of its own is not.
#[cfg(target_os = "linux")]
fn can_create_user_namespace() -> bool {
    // SAFETY: called before CEF starts, while this is the only thread, so the
    // child is a clean copy; it calls nothing but unshare and _exit.
    unsafe {
        let pid = libc::fork();
        if pid < 0 {
            return false;
        }
        if pid == 0 {
            let entered = libc::unshare(libc::CLONE_NEWUSER) == 0;
            libc::_exit(if entered { 0 } else { 1 });
        }
        let mut status = 0;
        libc::waitpid(pid, &mut status, 0) == pid
            && libc::WIFEXITED(status)
            && libc::WEXITSTATUS(status) == 0
    }
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
    fn hands_the_page_every_primary_modifier_chord() {
        // Flag values are CEF's own (cef_event_flags_t): shift 2, control 4,
        // alt 8, command 128.
        assert!(page_first(4), "Ctrl");
        assert!(page_first(4 | 2), "Ctrl+Shift");
        assert!(page_first(128), "Cmd");
        assert!(page_first(128 | 2), "Cmd+Shift");
    }

    #[test]
    fn leaves_the_rest_to_chromium() {
        assert!(!page_first(0), "no modifier");
        assert!(!page_first(2), "Shift");
        assert!(!page_first(8), "Alt");
        assert!(!page_first(2 | 8), "Shift+Alt");
    }

    #[test]
    fn splits_lich_switches_from_chromium_switches() {
        let launch = parse(args(&[
            "--url=http://127.0.0.1:47821/?token=x",
            "--user-data-dir=/home/u/.config/lich/chromium-profile",
            "--class=lich",
            "--no-first-run",
            "--disable-features=Translate",
            "--exit-on-stdin-eof",
            "--ozone-platform=wayland",
        ]));
        assert_eq!(
            launch,
            Launch {
                url: Some("http://127.0.0.1:47821/?token=x".into()),
                class: Some("lich".into()),
                profile_dir: Some("/home/u/.config/lich/chromium-profile".into()),
                exit_on_stdin_eof: true,
                switches: vec![
                    ("no-first-run".into(), None),
                    ("ozone-platform".into(), Some("wayland".into())),
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
    fn returns_once_stdin_ends_whatever_was_written() {
        wait_for_eof(std::io::Cursor::new(b"noise"));
        wait_for_eof(std::io::empty());
    }

    #[test]
    fn keeps_the_first_equals_inside_a_value() {
        let launch = parse(args(&["--url=http://h/?a=1&b=2"]));
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

    #[cfg(target_os = "linux")]
    #[test]
    fn tells_the_browser_process_from_its_subprocesses() {
        assert!(!parse(args(&["--url=http://h/", "--class=lich"])).is_subprocess());
        assert!(parse(args(&["--type=zygote", "--no-zygote-sandbox"])).is_subprocess());
    }

    #[cfg(target_os = "linux")]
    #[test]
    fn requires_the_helper_root_owned_setuid_and_world_executable() {
        assert!(helper_usable(0, 0o104755), "root, 4755");
        assert!(!helper_usable(1000, 0o104755), "not root");
        assert!(!helper_usable(0, 0o100755), "no setuid bit");
        assert!(!helper_usable(0, 0o104750), "not executable by others");
    }

    #[test]
    fn ignores_what_is_not_a_switch() {
        let launch = parse(args(&["run", "-x", "--", "http://h/"]));
        assert_eq!(launch, Launch::default());
    }
}

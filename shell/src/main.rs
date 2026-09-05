//! lich's window: an embedded Chromium (CEF, through kurogane) that the Go
//! backend launches exactly the way it launches a system browser. The argv is
//! the contract of `internal/chromium.Args`: `--app=<url>` is the page,
//! `--class=<name>` names the window for the window manager,
//! `--user-data-dir=<dir>` is where the profile lives, and every other
//! `--switch[=value]` is a Chromium switch to honour (`lich -- --ozone-platform=wayland`).
use kurogane::App;

/// The title the window carries. The page title is never used for it.
const TITLE: &str = "lich";

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
            _ => launch.switches.push((name.to_owned(), value)),
        }
    }
    launch
}

fn main() {
    // CEF re-executes this binary for the renderer, GPU and utility roles with
    // an argv of its own. Those roles exit inside run_or_exit before any window
    // exists, so a missing --app= is a subprocess, not an error.
    let launch = parse(std::env::args().skip(1));
    let mut app = App::url(launch.url.unwrap_or_else(|| "about:blank".into()))
        .profile_id("lich")
        .window_title(TITLE)
        // CEF's Chrome runtime loads the extensions a distribution installs
        // system-wide (plasma-browser-integration on KDE). A window that is
        // not a browser has no use for them.
        .chromium_flag("disable-extensions");
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
    // The user's switches go through kurogane rather than staying in argv, so
    // they land after its own policy and win: --ozone-platform=wayland beats
    // the x11 it forces on NVIDIA.
    for (name, value) in launch.switches {
        app = match value {
            Some(value) => app.chromium_flag_with_value(name, value),
            None => app.chromium_flag(name),
        };
    }
    app.run_or_exit();
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
                    ("disable-features".into(), Some("Translate".into())),
                ],
            }
        );
    }

    #[test]
    fn keeps_the_first_equals_inside_a_value() {
        let launch = parse(args(&["--app=http://h/?a=1&b=2"]));
        assert_eq!(launch.url.as_deref(), Some("http://h/?a=1&b=2"));
    }

    #[test]
    fn ignores_what_is_not_a_switch() {
        let launch = parse(args(&["run", "-x", "--", "http://h/"]));
        assert_eq!(launch, Launch::default());
    }
}

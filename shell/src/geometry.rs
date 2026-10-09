//! The window's placement, remembered between runs in a file of the profile.
//! Not in Chromium's own prefs: Chromium empties and recreates those when it
//! finds them damaged.
use kurogane::{WindowPlacement, WindowState};
use std::path::{Path, PathBuf};

/// The file, beside the Chromium profile it belongs to: `task dev` and every
/// other profile keep a window of their own.
pub fn file(profile_dir: &str) -> PathBuf {
    Path::new(profile_dir).join("lich-window")
}

/// One line, `x y width height maximized`. The format the file has had since
/// the window first remembered itself; the state is a flag because only
/// maximized is kept.
pub fn encode(placement: WindowPlacement) -> String {
    format!(
        "{} {} {} {} {}\n",
        placement.x,
        placement.y,
        placement.width,
        placement.height,
        u8::from(placement.state == WindowState::Maximized)
    )
}

pub fn decode(text: &str) -> Option<WindowPlacement> {
    let mut fields = text.split_whitespace().map(str::parse::<i64>);
    let mut next = || fields.next()?.ok();
    let (x, y, width, height, maximized) = (next()?, next()?, next()?, next()?, next()?);
    if !matches!(maximized, 0 | 1) || next().is_some() {
        return None;
    }
    Some(WindowPlacement {
        x: i32::try_from(x).ok()?,
        y: i32::try_from(y).ok()?,
        // A 0 side is a file from before kurogane sized the window itself, a
        // window only ever seen maximized; its restored size is anyone's
        // guess, and 1 is the smallest kurogane opens at, which a maximized
        // window never shows.
        width: u32::try_from(width).ok()?.max(1),
        height: u32::try_from(height).ok()?.max(1),
        state: if maximized == 1 {
            WindowState::Maximized
        } else {
            WindowState::Normal
        },
    })
}

/// What is saved when the window closes, given the work area of its display.
/// kurogane reports where a maximized window restores to, so the rectangle
/// is kept as it comes; only the state is read again. Hyprland configures
/// every window tiled on all four edges, floating ones included, and
/// Chromium reports that as maximized (measured, 0.56): a maximized window
/// spans its work area, one that does not is not. Fullscreen is not kept:
/// a window reopened fullscreen has no frame to leave it by.
pub fn closed(placement: WindowPlacement, work_area_width: u32) -> WindowPlacement {
    let state = match placement.state {
        WindowState::Maximized if work_area_width > 0 && placement.width < work_area_width => {
            WindowState::Normal
        }
        WindowState::Maximized => WindowState::Maximized,
        _ => WindowState::Normal,
    };
    WindowPlacement { state, ..placement }
}

#[cfg(test)]
mod tests {
    use super::*;

    const NORMAL: WindowPlacement = WindowPlacement {
        x: 100,
        y: 50,
        width: 1200,
        height: 800,
        state: WindowState::Normal,
    };

    #[test]
    fn round_trips_through_the_file() {
        let maximized = WindowPlacement {
            x: -1920,
            state: WindowState::Maximized,
            ..NORMAL
        };
        assert_eq!(decode(&encode(NORMAL)), Some(NORMAL));
        assert_eq!(decode(&encode(maximized)), Some(maximized));
        assert_eq!(encode(NORMAL), "100 50 1200 800 0\n");
    }

    #[test]
    fn rejects_a_damaged_file() {
        for text in [
            "",
            "1 2 3 4",
            "1 2 3 4 1 9",
            "1 2 x 4 0",
            "1 2 -3 4 0",
            "1 2 3 4 2",
            "99999999999 2 3 4 0",
        ] {
            assert_eq!(decode(text), None, "{text:?}");
        }
    }

    #[test]
    fn reads_a_file_from_before_the_window_had_a_size() {
        let placement = decode("0 0 0 0 1").unwrap();
        assert_eq!((placement.width, placement.height), (1, 1));
        assert_eq!(placement.state, WindowState::Maximized);
    }

    #[test]
    fn saves_a_maximized_state_that_does_not_span_the_work_area_as_normal() {
        let floating = WindowPlacement {
            width: 800,
            state: WindowState::Maximized,
            ..NORMAL
        };
        assert_eq!(closed(floating, 1920).state, WindowState::Normal);
        assert_eq!(
            closed(floating, 0).state,
            WindowState::Maximized,
            "no display known"
        );
        let spanning = WindowPlacement {
            width: 1920,
            ..floating
        };
        assert_eq!(closed(spanning, 1920).state, WindowState::Maximized);
    }

    #[test]
    fn keeps_the_rectangle_and_drops_fullscreen() {
        let fullscreen = WindowPlacement {
            state: WindowState::Fullscreen,
            ..NORMAL
        };
        assert_eq!(closed(fullscreen, 1920), NORMAL);
        assert_eq!(closed(NORMAL, 1920), NORMAL);
    }
}

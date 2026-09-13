//! The window's size, position and maximized state, remembered between runs
//! in a file of the profile. Not in Chromium's own prefs: Chromium empties and
//! recreates those when it finds them damaged.
use kurogane::{BrowserBounds, WindowState};
use std::path::{Path, PathBuf};

/// The file, beside the Chromium profile it belongs to: `task dev` and every
/// other profile keep a window of their own.
pub fn file(profile_dir: &str) -> PathBuf {
    Path::new(profile_dir).join("lich-window")
}

#[derive(Clone, Copy, Debug, PartialEq)]
pub struct Geometry {
    pub x: i32,
    pub y: i32,
    /// 0 when the window was only ever seen maximized: there is no normal size
    /// to restore, and an empty rectangle is CEF's own default size.
    pub width: i32,
    pub height: i32,
    pub maximized: bool,
}

impl Geometry {
    fn bounds(self) -> BrowserBounds {
        BrowserBounds {
            x: self.x,
            y: self.y,
            width: self.width,
            height: self.height,
        }
    }

    /// One line, `x y width height maximized`.
    pub fn encode(self) -> String {
        format!(
            "{} {} {} {} {}\n",
            self.x,
            self.y,
            self.width,
            self.height,
            u8::from(self.maximized)
        )
    }

    pub fn decode(text: &str) -> Option<Self> {
        let mut fields = text.split_whitespace().map(str::parse::<i32>);
        let mut next = || fields.next()?.ok();
        let (x, y, width, height, maximized) = (next()?, next()?, next()?, next()?, next()?);
        if width < 0 || height < 0 || !matches!(maximized, 0 | 1) || next().is_some() {
            return None;
        }
        Some(Self {
            x,
            y,
            width,
            height,
            maximized: maximized == 1,
        })
    }

    /// What the window opens with: the saved rectangle pulled onto `work_area`,
    /// the display nearest to it, so a window last seen on a monitor that is
    /// gone opens on one that is not.
    pub fn restore(self, work_area: BrowserBounds) -> (BrowserBounds, WindowState) {
        let state = if self.maximized {
            WindowState::Maximized
        } else {
            WindowState::Normal
        };
        // No display known is no reason to lose the rectangle, and an empty
        // area would leave clamp a minimum above its maximum.
        if self.width == 0 || self.height == 0 || work_area.width <= 0 || work_area.height <= 0 {
            return (self.bounds(), state);
        }
        let width = self.width.min(work_area.width);
        let height = self.height.min(work_area.height);
        let bounds = BrowserBounds {
            x: self
                .x
                .clamp(work_area.x, work_area.x + work_area.width - width),
            y: self
                .y
                .clamp(work_area.y, work_area.y + work_area.height - height),
            width,
            height,
        };
        (bounds, state)
    }

    /// What is saved when the window closes, given what was saved before. CEF
    /// has no restored bounds: a maximized window reports the maximized
    /// rectangle, so the normal one saved earlier is kept. A minimized window
    /// reports nothing worth keeping.
    pub fn closed(
        bounds: BrowserBounds,
        state: WindowState,
        previous: Option<Self>,
    ) -> Option<Self> {
        match state {
            WindowState::Normal => Some(Self {
                x: bounds.x,
                y: bounds.y,
                width: bounds.width,
                height: bounds.height,
                maximized: false,
            }),
            WindowState::Maximized => Some(Self {
                maximized: true,
                ..previous.unwrap_or(Self {
                    x: 0,
                    y: 0,
                    width: 0,
                    height: 0,
                    maximized: true,
                })
            }),
            WindowState::Minimized | WindowState::Hidden => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn rect(x: i32, y: i32, width: i32, height: i32) -> BrowserBounds {
        BrowserBounds {
            x,
            y,
            width,
            height,
        }
    }

    fn fields(b: BrowserBounds) -> (i32, i32, i32, i32) {
        (b.x, b.y, b.width, b.height)
    }

    const NORMAL: Geometry = Geometry {
        x: 100,
        y: 50,
        width: 1200,
        height: 800,
        maximized: false,
    };

    #[test]
    fn round_trips_through_the_file() {
        let maximized = Geometry {
            x: -1920,
            maximized: true,
            ..NORMAL
        };
        assert_eq!(Geometry::decode(&NORMAL.encode()), Some(NORMAL));
        assert_eq!(Geometry::decode(&maximized.encode()), Some(maximized));
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
        ] {
            assert_eq!(Geometry::decode(text), None, "{text:?}");
        }
    }

    #[test]
    fn restores_a_rectangle_that_fits_as_it_was() {
        let (bounds, state) = NORMAL.restore(rect(0, 0, 2560, 1440));
        assert_eq!(fields(bounds), (100, 50, 1200, 800));
        assert_eq!(state, WindowState::Normal);
    }

    #[test]
    fn pulls_a_window_from_an_unplugged_monitor_onto_the_nearest_display() {
        let gone = Geometry { x: 3000, ..NORMAL };
        let (bounds, _) = gone.restore(rect(0, 0, 1920, 1080));
        assert_eq!(fields(bounds), (720, 50, 1200, 800));
        let left = Geometry {
            x: -2500,
            y: -40,
            ..NORMAL
        };
        let (bounds, _) = left.restore(rect(0, 30, 1920, 1050));
        assert_eq!(fields(bounds), (0, 30, 1200, 800));
    }

    #[test]
    fn shrinks_a_window_larger_than_the_display() {
        let big = Geometry {
            width: 3840,
            height: 2160,
            ..NORMAL
        };
        let (bounds, _) = big.restore(rect(1920, 0, 1920, 1040));
        assert_eq!(fields(bounds), (1920, 0, 1920, 1040));
    }

    #[test]
    fn opens_maximized_at_the_default_size_when_no_normal_size_is_known() {
        let only_maximized = Geometry::closed(rect(0, 0, 1920, 1080), WindowState::Maximized, None);
        let (bounds, state) = only_maximized.unwrap().restore(rect(0, 0, 1920, 1080));
        assert_eq!(fields(bounds), (0, 0, 0, 0));
        assert_eq!(state, WindowState::Maximized);
    }

    #[test]
    fn keeps_the_normal_rectangle_when_closed_maximized() {
        let saved = Geometry::closed(rect(0, 0, 1920, 1080), WindowState::Maximized, Some(NORMAL));
        assert_eq!(
            saved,
            Some(Geometry {
                maximized: true,
                ..NORMAL
            })
        );
    }

    #[test]
    fn saves_the_normal_rectangle_and_ignores_a_minimized_close() {
        let saved = Geometry::closed(rect(10, 20, 900, 700), WindowState::Normal, Some(NORMAL));
        assert_eq!(
            saved,
            Some(Geometry {
                x: 10,
                y: 20,
                width: 900,
                height: 700,
                maximized: false,
            })
        );
        assert_eq!(
            Geometry::closed(rect(0, 0, 0, 0), WindowState::Minimized, Some(NORMAL)),
            None
        );
    }
}

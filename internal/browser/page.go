package browser

import "context"

// Handle is what Open and OpenVisible return: the id later calls take, and
// enough for a card or a tool result to say which window it is.
type Handle struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Owner  string `json:"owner"`
	Headed bool   `json:"headed"`
}

// Target picks an element for click and type. Index is the one page-info
// stamped (data-lich-idx); selector is a CSS query; x/y are viewport
// coordinates. Coordinates win when set, then selector, then index.
type Target struct {
	Index    *int     `json:"index,omitempty"`
	Selector string   `json:"selector,omitempty"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
}

func (t Target) empty() bool {
	return t.Index == nil && t.Selector == "" && (t.X == nil || t.Y == nil)
}

// Rect is an element's viewport box, integer pixels.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Interactive is one clickable or fillable node from page-info. It never
// carries a control's value — filled/checked say that typing landed without
// putting the content in the transcript.
type Interactive struct {
	Index   int    `json:"index"`
	Tag     string `json:"tag"`
	Type    string `json:"type,omitempty"`
	Role    string `json:"role,omitempty"`
	Text    string `json:"text"`
	Href    string `json:"href,omitempty"`
	Rect    Rect   `json:"rect"`
	Filled  *bool  `json:"filled,omitempty"`
	Checked *bool  `json:"checked,omitempty"`
}

// PageInfo is what the agent reads instead of a screenshot: the URL, the
// visible text, and the interactive nodes it can click or type into.
type PageInfo struct {
	URL         string        `json:"url"`
	Title       string        `json:"title"`
	Text        string        `json:"text"`
	Interactive []Interactive `json:"interactive"`
	Truncated   bool          `json:"truncated"`
}

// Page is one Chromium tab the service drives. chrome.go is the real one;
// tests inject a fake.
type Page interface {
	Navigate(ctx context.Context, url string) error
	Info(ctx context.Context) (PageInfo, error)
	Click(ctx context.Context, t Target) error
	Type(ctx context.Context, text string, clear bool, t Target) error
	Press(ctx context.Context, key string, t Target) error
	Screenshot(ctx context.Context, dest string) error
	Reload(ctx context.Context) error
	Back(ctx context.Context) error
	Forward(ctx context.Context) error
	Scroll(ctx context.Context, dy int) error
	Focus(ctx context.Context) error
	Alive() bool
	Close() error
}

// Launcher starts a Chromium with its own user-data-dir. headed is a real
// window; false is headless. The directory is the caller's to delete.
type Launcher func(dir string, headed bool) (Page, error)

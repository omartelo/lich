package cli

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/omartelo/lich/internal/browser"
)

func (c *client) browser(args []string) error {
	if len(args) == 0 {
		return usageError("browser")
	}
	if args[0] == "-h" || args[0] == "--help" {
		printHelp(c.stdout, newFlagSet("browser"))
		return errHelpShown
	}
	switch args[0] {
	case "open":
		return c.browserOpen(args[1:])
	case "info":
		return c.browserInfo(args[1:])
	case "click":
		return c.browserClick(args[1:])
	case "type":
		return c.browserType(args[1:])
	case "press":
		return c.browserPress(args[1:])
	case "screenshot":
		return c.browserScreenshot(args[1:])
	case "navigate":
		return c.browserNavigate(args[1:])
	case "reload":
		return c.browserSimple("browser.Reload", args[1:])
	case "back":
		return c.browserSimple("browser.Back", args[1:])
	case "forward":
		return c.browserSimple("browser.Forward", args[1:])
	case "scroll":
		return c.browserScroll(args[1:])
	case "list":
		return c.browserList(args[1:])
	case "close":
		return c.browserSimple("browser.Close", args[1:])
	default:
		return usageError("browser")
	}
}

func (c *client) browserOpen(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	var h browser.Handle
	if err := c.call("browser.Open", []any{owner, strings.Join(flags.Args(), " ")}, openCall, &h); err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, h.URL)
	return nil
}

func (c *client) browserInfo(args []string) error {
	if err := c.browserNoArgs(args); err != nil {
		return err
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	var info browser.PageInfo
	if err := c.call("browser.Info", []any{owner}, openCall, &info); err != nil {
		return err
	}
	return c.emit(info)
}

func (c *client) browserClick(args []string) error {
	flags := newFlagSet("browser")
	tf := registerTarget(flags)
	if err := c.parse(flags, args); err != nil {
		return err
	}
	t := tf.target()
	if t.Index == nil && t.Selector == "" && (t.X == nil || t.Y == nil) {
		return fmt.Errorf("click needs --index, --selector, or --x and --y")
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	return c.call("browser.Click", []any{owner, t}, shortCall, nil)
}

func (c *client) browserType(args []string) error {
	flags := newFlagSet("browser")
	clear := flags.Bool("clear", false, "replace the field before typing")
	tf := registerTarget(flags)
	if err := c.parse(flags, args); err != nil {
		return err
	}
	text := strings.Join(flags.Args(), " ")
	if text == "" {
		return usageError("browser")
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	return c.call("browser.Type", []any{owner, text, *clear, tf.target()}, shortCall, nil)
}

func (c *client) browserPress(args []string) error {
	flags := newFlagSet("browser")
	tf := registerTarget(flags)
	if err := c.parse(flags, args); err != nil {
		return err
	}
	key := strings.Join(flags.Args(), " ")
	if key == "" {
		return usageError("browser")
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	return c.call("browser.Press", []any{owner, key, tf.target()}, shortCall, nil)
}

func (c *client) browserScreenshot(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	dest := ""
	if flags.NArg() > 0 {
		dest = flags.Arg(0)
	}
	var path string
	if err := c.call("browser.Screenshot", []any{owner, dest}, openCall, &path); err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, path)
	return nil
}

func (c *client) browserNavigate(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	url := strings.Join(flags.Args(), " ")
	if url == "" {
		return usageError("browser")
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	var h browser.Handle
	if err := c.call("browser.Navigate", []any{owner, url}, openCall, &h); err != nil {
		return err
	}
	fmt.Fprintln(c.stdout, h.URL)
	return nil
}

func (c *client) browserScroll(args []string) error {
	flags := newFlagSet("browser")
	dy := flags.Int("dy", 400, "pixels to scroll (positive is down)")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	return c.call("browser.Scroll", []any{owner, *dy}, shortCall, nil)
}

func (c *client) browserList(args []string) error {
	if err := c.browserNoArgs(args); err != nil {
		return err
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	var list []browser.Handle
	if err := c.call("browser.List", []any{owner}, shortCall, &list); err != nil {
		return err
	}
	if list == nil {
		list = []browser.Handle{}
	}
	return c.emit(list)
}

func (c *client) browserSimple(method string, args []string) error {
	if err := c.browserNoArgs(args); err != nil {
		return err
	}
	owner, err := c.browserOwner()
	if err != nil {
		return err
	}
	return c.call(method, []any{owner}, shortCall, nil)
}

func (c *client) browserNoArgs(args []string) error {
	flags := newFlagSet("browser")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("browser")
	}
	return nil
}

type targetFlags struct {
	index    *int
	selector *string
	x, y     *float64
}

func registerTarget(flags *flag.FlagSet) *targetFlags {
	t := &targetFlags{}
	t.selector = flags.String("selector", "", "CSS selector")
	flags.Func("index", "element index from browser info", func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		t.index = &n
		return nil
	})
	flags.Func("x", "viewport X", func(s string) error {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		t.x = &n
		return nil
	})
	flags.Func("y", "viewport Y", func(s string) error {
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		t.y = &n
		return nil
	})
	return t
}

func (t *targetFlags) target() browser.Target {
	sel := ""
	if t.selector != nil {
		sel = *t.selector
	}
	return browser.Target{Index: t.index, Selector: sel, X: t.x, Y: t.y}
}

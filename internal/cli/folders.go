package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/omartelo/lich/internal/relay"
	"github.com/omartelo/lich/internal/spawn"
)

// The sidebar's folders, on both surfaces: `lich folders`, `lich file` and
// `lich rename-folder`, and the MCP tools that say the same thing. A folder is a
// name on a session and nothing else (store.SetSessionFolder), so what an agent
// can do with one is what the window's menus do: file a card, take it out,
// rename the folder or take it apart.

func (c *client) folders(args []string) error {
	flags := newFlagSet("folders")
	project := flags.String(
		"project", "", "project to list, by name or by directory path; defaults to the caller's own",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("folders")
	}

	var folders []spawn.Folder
	if err := c.call(context.Background(), "spawn.Folders", []any{c.sessionID(), *project}, shortCall, &folders); err != nil {
		return err
	}
	folders = asList(folders)
	if *asJSON {
		return c.emit(folders)
	}
	if len(folders) == 0 {
		fmt.Fprintln(c.stdout, "No folders.")
		return nil
	}
	fmt.Fprintln(c.stdout, "folder\tsessions")
	for _, folder := range folders {
		fmt.Fprintf(c.stdout, "%s\t%s\n", folder.Name, strings.Join(folder.Sessions, ", "))
	}
	return nil
}

// file takes one argument or two, as rename does: the folder alone files the
// session the command runs in; a target before it files that one.
func (c *client) file(args []string) error {
	flags := newFlagSet("file")
	project := flags.String(
		"project", "",
		"narrow the target to one project, by name or by directory path, when the label is ambiguous",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	var target, folder string
	switch flags.NArg() {
	case 1:
		folder = flags.Arg(0)
	case 2:
		target, folder = flags.Arg(0), flags.Arg(1)
	default:
		return usageError("file")
	}

	var filed spawn.Filed
	call := []any{c.sessionID(), target, *project, folder}
	if err := c.call(context.Background(), "spawn.File", call, shortCall, &filed); err != nil {
		return err
	}
	if *asJSON {
		return c.emit(filed)
	}
	fmt.Fprint(c.stdout, filedText(filed))
	return nil
}

func (c *client) renameFolder(args []string) error {
	flags := newFlagSet("rename-folder")
	project := flags.String(
		"project", "", "project the folder is in, by name or by directory path; defaults to the caller's own",
	)
	asJSON := flags.Bool("json", false, "print the result as JSON")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return usageError("rename-folder")
	}

	var refiled spawn.Refiled
	call := []any{c.sessionID(), *project, flags.Arg(0), flags.Arg(1)}
	if err := c.call(context.Background(), "spawn.RenameFolder", call, shortCall, &refiled); err != nil {
		return err
	}
	if *asJSON {
		return c.emit(refiled)
	}
	fmt.Fprint(c.stdout, refiledText(refiled))
	return nil
}

// filedText says where the session is now, and where it came out of when the
// answer is that it is in no folder.
func filedText(filed spawn.Filed) string {
	switch {
	case filed.Folder != "":
		return fmt.Sprintf("Filed %q under %q.\n", filed.Label, filed.Folder)
	case filed.Previous != "":
		return fmt.Sprintf("Took %q out of %q.\n", filed.Label, filed.Previous)
	default:
		return fmt.Sprintf("%q was in no folder.\n", filed.Label)
	}
}

// refiledText names every session that moved: a rename onto a folder the
// project already had is a merge, and the list is the only thing that says so.
func refiledText(refiled spawn.Refiled) string {
	moved := relay.QuotedList(refiled.Sessions)
	if refiled.To == "" {
		return fmt.Sprintf("Took folder %q apart, taking out %s.\n", refiled.From, moved)
	}
	return fmt.Sprintf("Moved %s from folder %q to %q.\n", moved, refiled.From, refiled.To)
}

// folderTools are the folder commands as MCP tools, appended to mcpTools.
var folderTools = []mcpTool{
	{
		Name: "list_folders",
		Description: "The sidebar folders of a project: each folder's name and the sessions " +
			"filed under it, by label. A folder groups sessions by the work they are on rather " +
			"than the checkout they live in, and exists only while a session is filed under it. " +
			"Use it before filing a session, to reuse a name exactly: names are matched as " +
			"written, so \"apps\" beside \"Apps\" is a second folder.",
		Schema: schema(map[string]any{
			"project": property("string",
				"Project to list, by name or by directory path. Defaults to your own."),
		}),
		ReadOnly: true,
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			var folders []spawn.Folder
			call := []any{c.sessionID(), args.text("project")}
			if err := c.call(ctx, "spawn.Folders", call, shortCall, &folders); err != nil {
				return "", err
			}
			out, err := json.Marshal(asList(folders))
			if err != nil {
				return "", fmt.Errorf("encode the folders: %w", err)
			}
			return string(out), nil
		},
	},
	{
		Name: "file_session",
		Description: "Move a lich session into a sidebar folder, the window's \"Move to " +
			"folder\". Omit the session to file your own. A session is in at most one folder, " +
			"so filing it moves it; a folder no session carries yet starts existing with this " +
			"one in it, and an empty folder takes the session out of the one it is in.",
		Schema: schema(map[string]any{
			"folder": property("string",
				"Folder to file the session under, exactly as list_folders names it, or a new "+
					"name. An empty string takes the session out of its folder."),
			"session": property("string",
				"Session to file, by the label on its card or the name it answers to. Omit to "+
					"file the session you are running in."),
			"project": property("string",
				"Project to narrow to, by name or by directory path, when the same label "+
					"exists in more than one."),
		}, "folder"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			// Required here too, because the empty name means something: a call
			// that left the folder out would take the session out of its own.
			if _, named := args["folder"]; !named {
				return "", errors.New("name the folder to file the session under; " +
					"an empty one takes the session out of the folder it is in")
			}
			var filed spawn.Filed
			call := []any{c.sessionID(), args.text("session"), args.text("project"), args.text("folder")}
			if err := c.call(ctx, "spawn.File", call, shortCall, &filed); err != nil {
				return "", err
			}
			return filedText(filed), nil
		},
	},
	{
		Name: "rename_folder",
		Description: "Rename a sidebar folder across every session filed under it (the " +
			"window's \"Rename folder\"), or, with an empty new name, take it apart: its " +
			"sessions go back among their checkout's cards. Renaming onto a name the project " +
			"already has merges the two folders. Returns every session that moved.",
		Schema: schema(map[string]any{
			"folder": property("string", "The folder to rename, exactly as list_folders names it."),
			"to":     property("string", "Its new name. An empty string takes the folder apart."),
			"project": property("string",
				"Project the folder is in, by name or by directory path. Defaults to your own."),
		}, "folder", "to"),
		Run: func(ctx context.Context, c *client, args mcpArgs) (string, error) {
			// For file_session's reason: an empty new name takes the folder apart.
			if _, named := args["to"]; !named {
				return "", errors.New("name the folder's new name; an empty one takes the folder apart")
			}
			var refiled spawn.Refiled
			call := []any{c.sessionID(), args.text("project"), args.text("folder"), args.text("to")}
			if err := c.call(ctx, "spawn.RenameFolder", call, shortCall, &refiled); err != nil {
				return "", err
			}
			return refiledText(refiled), nil
		},
	},
}

package patchnotes

import (
	"reflect"
	"testing"
)

const sample = `# Changelog

Some preamble.

## [Unreleased]

### Added

- Nothing yet.

## [0.11.0] - 2026-07-21

### Added

- **Default provider for new sessions.** Pick which harness a new worktree,
  the new-session hotkey, and a project's first session spawn.
- **Command palette.** Jump to any session from anywhere.

### Fixed

- **Reopening a worktree no longer forks a new one** from its branch.

## [0.10.0] - 2026-07-01

### Added

- An older feature.
`

func TestSectionParsesGroupsAndFoldsWrappedItems(t *testing.T) {
	got := Section(sample, "v0.11.0")
	want := []Group{
		{Label: "Added", Items: []string{
			"**Default provider for new sessions.** Pick which harness a new worktree, the new-session hotkey, and a project's first session spawn.",
			"**Command palette.** Jump to any session from anywhere.",
		}},
		{Label: "Fixed", Items: []string{
			"**Reopening a worktree no longer forks a new one** from its branch.",
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Section mismatch:\n got %#v\nwant %#v", got, want)
	}
}

func TestSectionMatchesRegardlessOfLeadingV(t *testing.T) {
	withV := Section(sample, "v0.11.0")
	withoutV := Section(sample, "0.11.0")
	if !reflect.DeepEqual(withV, withoutV) {
		t.Fatalf("leading v changed the result: %#v vs %#v", withV, withoutV)
	}
}

func TestSectionStopsAtNextVersion(t *testing.T) {
	// 0.11.0 must not bleed into 0.10.0's "An older feature." item.
	got := Section(sample, "0.11.0")
	for _, g := range got {
		for _, item := range g.Items {
			if item == "An older feature." {
				t.Fatalf("section leaked into the next release")
			}
		}
	}
}

func TestSectionMissingVersionIsNil(t *testing.T) {
	if got := Section(sample, "9.9.9"); got != nil {
		t.Fatalf("want nil for absent version, got %#v", got)
	}
	if got := Section(sample, "dev"); got != nil {
		t.Fatalf("want nil for dev build, got %#v", got)
	}
}

func TestCurrentReportsNormalizedVersion(t *testing.T) {
	svc := New("v0.11.0", sample)
	got := svc.Current()
	if got.Version != "0.11.0" {
		t.Fatalf("Version = %q, want 0.11.0", got.Version)
	}
	if len(got.Groups) != 2 {
		t.Fatalf("Groups = %d, want 2", len(got.Groups))
	}
}

func TestSectionDropsEmptyGroups(t *testing.T) {
	const cl = `## [1.0.0]

### Added

### Changed

- A real change.
`
	got := Section(cl, "1.0.0")
	want := []Group{{Label: "Changed", Items: []string{"A real change."}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("empty group not dropped:\n got %#v\nwant %#v", got, want)
	}
}

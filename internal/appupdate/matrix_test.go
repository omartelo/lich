package appupdate

import (
	"os"
	"path/filepath"
	"testing"
)

// TestUpgradeMatrix is every install layout a release ships, laid out on disk,
// against the route an update from it takes. Every release asset is a complete
// package, binary and window together, so each row is one of them. A layout
// change that moves a route fails here instead of on somebody's machine.
func TestUpgradeMatrix(t *testing.T) {
	const (
		debian = "ID=debian\n"
		fedora = "ID=fedora\nID_LIKE=\"rhel centos\"\n"
		arch   = "ID=arch\n"
	)
	cask := "paste brew upgrade --cask omartelo/tap/lich" + restartChain
	scoop := "paste scoop update lich" + restartChainPwsh
	packaged := []string{"usr/lib/lich/shell/lich-shell"}

	rows := []struct {
		name      string
		goos      string
		goarch    string
		layout    []string // files beside or above the exe, relative to a temp root
		exe       string   // relative to the same root
		osRelease string
		want      string
	}{
		{"windows installer", "windows", "amd64", []string{"lich/unins000.exe", "lich/shell/lich-shell.exe"}, "lich/lich.exe", "", "installer"},
		{"windows portable zip", "windows", "amd64", []string{"lich/shell/lich-shell.exe"}, "lich/lich.exe", "", "installer"},
		{"scoop, current junction", "windows", "amd64", []string{"scoop/apps/lich/current/shell/lich-shell.exe"}, "scoop/apps/lich/current/lich.exe", "", scoop},
		{"scoop, version directory", "windows", "amd64", []string{"scoop/apps/lich/0.50.0/shell/lich-shell.exe"}, "scoop/apps/lich/0.50.0/lich.exe", "", scoop},
		{"deb", "linux", "amd64", packaged, "usr/bin/lich", debian, "paste " + installScript},
		{"rpm", "linux", "amd64", packaged, "usr/bin/lich", fedora, "paste " + installScript},
		{"aur lich-bin", "linux", "amd64", packaged, "usr/bin/lich", arch, "paste yay -S lich-bin" + restartChain},
		// A package installed over a tarball would leave /restart relaunching the
		// tarball's lich, so the tarball gets the release page, on Arch too.
		{"linux tarball", "linux", "amd64", []string{"opt/lich/shell/lich-shell"}, "opt/lich/lich", debian, "release page"},
		{"linux tarball on arch", "linux", "amd64", []string{"opt/lich/shell/lich-shell"}, "opt/lich/lich", arch, "release page"},
		{"cask, apple silicon", "darwin", "arm64", []string{"Applications/Lich.app/Contents/MacOS/lich-shell"}, "Applications/Lich.app/Contents/MacOS/lich", "", cask},
		{"cask, intel", "darwin", "amd64", nil, "Applications/Lich.app/Contents/MacOS/lich", "", cask},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			root := t.TempDir()
			for _, rel := range append(row.layout, row.exe) {
				touch(t, filepath.Join(root, filepath.FromSlash(rel)))
			}
			s := &Service{
				goos:          row.goos,
				goarch:        row.goarch,
				exePath:       filepath.Join(root, filepath.FromSlash(row.exe)),
				osReleasePath: filepath.Join(root, "os-release"),
			}
			if row.osRelease != "" {
				if err := os.WriteFile(s.osReleasePath, []byte(row.osRelease), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got := route(s); got != row.want {
				t.Errorf("route = %q, want %q", got, row.want)
			}
		})
	}
}

// route names what an update does on s: run the installer, paste a package
// manager's command, or send the user to the release page.
func route(s *Service) string {
	if canSelfApply(s.goos, s.goarch, s.exePath) {
		return "installer"
	}
	if cmd := s.installCommand(); cmd != "" {
		return "paste " + cmd
	}
	return "release page"
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o755); err != nil {
		t.Fatal(err)
	}
}

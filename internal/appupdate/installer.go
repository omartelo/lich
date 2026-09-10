package appupdate

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// The installer includes Chromium, several hundred MiB larger than the exe.
const installerLimit = 1 << 30

// installerUpdate reports the one install shape Inno Setup owns: a Windows
// layout the installer left its mark on and Scoop does not track, which only
// ships for amd64. The arch belongs in this predicate and not beside the asset
// name: Apply routes on it alone, so an arch published later would otherwise
// hand its bare exe to the installer and run it with /SILENT and /DIR=.
func (s *Service) installerUpdate() bool {
	return s.goos == "windows" && s.goarch == "amd64" &&
		installerOwned(s.exePath) && !scoopOwned(s.exePath)
}

func (s *Service) assetName(version string) string {
	if s.installerUpdate() {
		return "lich-v" + version + "-windows-amd64-setup.exe"
	}
	return assetName(s.goos, s.goarch, version)
}

func (s *Service) applyInstaller(r io.Reader, checksum []byte) error {
	if s.install == nil {
		return fmt.Errorf("installer updates unavailable")
	}
	// One retained installer, replaced on the next update; no growing cache of CEF.
	path := filepath.Join(filepath.Dir(s.exePath), ".lich-update-setup.exe")
	if err := stageInstaller(r, checksum, path, installerLimit); err != nil {
		return fmt.Errorf("verify installer: %w", err)
	}
	return s.install(path)
}

// limit is a parameter rather than the const so a test can reach the refusal
// without staging a gibibyte.
func stageInstaller(r io.Reader, checksum []byte, path string, limit int64) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".lich-download-*.exe")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, limit+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > limit {
		return fmt.Errorf("installer exceeds %d bytes", limit)
	}
	if !bytes.Equal(hash.Sum(nil), checksum) {
		return fmt.Errorf("installer checksum mismatch")
	}
	return os.Rename(f.Name(), path)
}

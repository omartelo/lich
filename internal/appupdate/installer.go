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

func (s *Service) installerUpdate() bool {
	return s.goos == "windows" && windowed(s.exePath)
}

func (s *Service) assetName(version string) string {
	if s.installerUpdate() && s.goarch == "amd64" {
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
	if err := stageInstaller(r, checksum, path); err != nil {
		return fmt.Errorf("verify installer: %w", err)
	}
	return s.install(path)
}

func stageInstaller(r io.Reader, checksum []byte, path string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".lich-download-*.exe")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, installerLimit+1))
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if n > installerLimit {
		return fmt.Errorf("installer exceeds %d bytes", installerLimit)
	}
	if !bytes.Equal(hash.Sum(nil), checksum) {
		return fmt.Errorf("installer checksum mismatch")
	}
	return os.Rename(f.Name(), path)
}

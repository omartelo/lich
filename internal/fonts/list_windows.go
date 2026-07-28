package fonts

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// fontsKeyPath is where Windows records every installed font. The value name
// carries the family and face ("Arial Bold (TrueType)"), the data the file.
const fontsKeyPath = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`

// listFamilies enumerates installed fonts from the registry — Windows has no
// fontconfig. Machine-wide and per-user fonts sit under the same path in
// different hives, and both feed the DirectWrite database Chromium resolves
// against.
func listFamilies() ([]string, error) {
	machine, err := fontValueNames(registry.LOCAL_MACHINE)
	if err != nil {
		return nil, fmt.Errorf("read installed fonts: %w", err)
	}
	user, err := fontValueNames(registry.CURRENT_USER)
	if err != nil {
		return nil, fmt.Errorf("read per-user fonts: %w", err)
	}
	return parseRegistryFamilies(append(machine, user...)), nil
}

// fontValueNames returns every value name under hive's Fonts key. A user who
// never installed a font for themselves has no key at all, which is an empty
// list rather than a failure.
func fontValueNames(hive registry.Key) ([]string, error) {
	key, err := registry.OpenKey(hive, fontsKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer key.Close()
	// A negative count asks for every value; the key holds one per installed face.
	return key.ReadValueNames(-1)
}

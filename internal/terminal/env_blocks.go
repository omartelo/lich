package terminal

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// KERN_PROCARGS2 returns argc, the executable path, NUL padding, argc argv
// strings, then environ. Never mistake an argument containing '=' for a login.
// https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_sysctl.c
func parseProcArgsEnv(data []byte) map[string]string {
	const argcBytes = 4
	if len(data) < argcBytes {
		return nil
	}
	argc := int32(binary.LittleEndian.Uint32(data))
	data = data[argcBytes:]
	if argc <= 0 || int64(argc) > int64(len(data)) {
		return nil
	}
	end := bytes.IndexByte(data, 0)
	if end <= 0 {
		return nil
	}
	data = bytes.TrimLeft(data[end:], "\x00")
	for range argc {
		end = bytes.IndexByte(data, 0)
		if end < 0 {
			return nil
		}
		data = data[end+1:]
	}
	return parseEnvBlock(string(data))
}

// Windows' environment is UTF-16LE, including a terminating empty string.
func parseUTF16Env(data []byte) map[string]string {
	if len(data)%2 != 0 {
		return nil
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2:])
	}
	for i := 0; i < len(units); i++ {
		if !utf16.IsSurrogate(rune(units[i])) {
			continue
		}
		if i+1 == len(units) || utf16.DecodeRune(rune(units[i]), rune(units[i+1])) == '\uFFFD' {
			return nil
		}
		i++
	}
	return parseEnvBlock(string(utf16.Decode(units)))
}

func parseEnvBlock(data string) map[string]string {
	env := make(map[string]string)
	for {
		entry, rest, terminated := strings.Cut(data, "\x00")
		if !terminated {
			return nil
		}
		if entry == "" {
			return env
		}
		// Windows stores drive working directories as =C:=C:\path.
		key, value, ok := strings.Cut(strings.TrimPrefix(entry, "="), "=")
		if !ok || key == "" {
			return nil
		}
		if !strings.HasPrefix(entry, "=") {
			env[key] = value
		}
		data = rest
	}
}

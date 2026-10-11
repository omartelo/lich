package lichclient

import (
	"fmt"
	"strconv"
)

// RuntimeFromEnv reads the backend a launching lich hands its window in the
// environment: `lich native` sets LICH_PORT and LICH_TOKEN. getenv is
// os.Getenv outside tests.
func RuntimeFromEnv(getenv func(string) string) (Runtime, error) {
	port, token := getenv("LICH_PORT"), getenv("LICH_TOKEN")
	if port == "" || token == "" {
		return Runtime{}, fmt.Errorf("no lich backend: want LICH_PORT and LICH_TOKEN in the environment (as `lich native` sets them) or -runtime pointing at a runtime file")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p <= 0 {
		return Runtime{}, fmt.Errorf("LICH_PORT=%q: want a port number", port)
	}
	return Runtime{Port: p, Token: token}, nil
}

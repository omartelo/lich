package lichclient

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestLiveShellSession drives a real backend: open a project, start a shell
// session, type into it over /ws and read the echo back. It needs a running
// lich, so it runs only when LICH_RUNTIME names its runtime file and
// LICH_LIVE_PROJECT a git checkout to open.
func TestLiveShellSession(t *testing.T) {
	runtimeFile, projectPath := os.Getenv("LICH_RUNTIME"), os.Getenv("LICH_LIVE_PROJECT")
	if runtimeFile == "" || projectPath == "" {
		t.Skip("set LICH_RUNTIME and LICH_LIVE_PROJECT to run against a live backend")
	}
	rt, err := ReadRuntime(runtimeFile)
	if err != nil {
		t.Fatal(err)
	}
	c := New(rt)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := c.AddProject(ctx, "live-test", "lich-demo", projectPath); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var out bytes.Buffer
	stream, _, err := c.Terminal(ctx, func(id string, data []byte) {
		if id == "live-shell" {
			mu.Lock()
			out.Write(data)
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddSession(ctx, "live-test", "live-shell", "Live shell", "shell", projectPath, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(ctx, "live-shell", "live-test", projectPath, "shell", 80, 24); err != nil {
		t.Fatal(err)
	}
	if err := stream.Send(ctx, "live-shell", []byte("echo native-$((40+2))\r")); err != nil {
		t.Fatal(err)
	}
	for ctx.Err() == nil {
		mu.Lock()
		got := out.String()
		mu.Unlock()
		if strings.Contains(got, "native-42") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if ctx.Err() != nil {
		t.Fatalf("no echo within the deadline; output so far: %q", out.String())
	}
	projects, err := c.LoadState(ctx)
	if err != nil || len(projects) == 0 || len(projects[0].Sessions) == 0 {
		t.Fatalf("LoadState = %+v, %v; want the project with its session", projects, err)
	}
}

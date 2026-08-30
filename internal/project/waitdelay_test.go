package project

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"
)

// helperEnv selects which half of the process tree a helper run plays.
const helperEnv = "LICH_WAITDELAY_HELPER"

// TestSubprocessHelper is not a test of anything: it is the child and the
// grandchild TestSubprocessBudgetOutlivesAGrandchild spawns, re-executing this
// binary because a shell one-liner would not run on Windows. As "child" it
// starts a grandchild on its own stdout — the pipe the parent is reading — and
// exits at once; as "grandchild" it holds that pipe open far past any budget
// the parent could set. A plain run does nothing.
func TestSubprocessHelper(t *testing.T) {
	switch os.Getenv(helperEnv) {
	case "child":
		grandchild := exec.Command(os.Args[0], "-test.run=TestSubprocessHelper")
		grandchild.Env = append(os.Environ(), helperEnv+"=grandchild")
		grandchild.Stdout = os.Stdout
		if err := grandchild.Start(); err != nil {
			t.Fatalf("start grandchild: %v", err)
		}
		os.Exit(0)
	case "grandchild":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
}

// A budget is only real if it bounds the call rather than the child. The child
// here exits immediately and the grandchild behind it keeps stdout open for ten
// seconds: with nothing closing that pipe, Output waits all ten and reports
// success, deadline or none.
func TestSubprocessBudgetOutlivesAGrandchild(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cmd := commandContext(ctx, os.Args[0], "-test.run=TestSubprocessHelper")
	cmd.Env = append(cmd.Env, helperEnv+"=child")

	start := time.Now()
	_, err := cmd.Output()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("Output blocked %s on a grandchild holding the pipe, err %v: the 200ms budget bounded the child alone", elapsed, err)
	}
}

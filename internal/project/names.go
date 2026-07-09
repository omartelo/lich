package project

import (
	"fmt"
	"math/rand/v2"
)

// Word pools for random worktree names, docker-style adjective-noun pairs.
var (
	adjectives = [...]string{
		"amber", "brave", "calm", "dusty", "eager", "fuzzy", "gentle", "icy",
		"jolly", "lucky", "mellow", "noble", "quiet", "rapid", "sunny", "witty",
	}
	nouns = [...]string{
		"badger", "comet", "dune", "ember", "falcon", "glacier", "harbor", "lynx",
		"meadow", "otter", "pine", "quartz", "raven", "sparrow", "tundra", "willow",
	}
)

// maxNameTries bounds the random draws before falling back to numeric suffixes.
const maxNameTries = 8

// randomWorktreeName picks an adjective-noun name that exists reports as free.
// After maxNameTries collisions it appends an incrementing suffix to the last
// draw, so it always terminates even with every pair taken.
func randomWorktreeName(exists func(string) bool) string {
	var name string
	for range maxNameTries {
		name = adjectives[rand.IntN(len(adjectives))] + "-" + nouns[rand.IntN(len(nouns))]
		if !exists(name) {
			return name
		}
	}
	for i := 2; ; i++ {
		if suffixed := fmt.Sprintf("%s-%d", name, i); !exists(suffixed) {
			return suffixed
		}
	}
}

package main

import "testing"

func TestGnoDoc(t *testing.T) {
	tc := []testMainCase{
		{
			args:                []string{"doc", "io.Writer"},
			stdoutShouldContain: "Writer is the interface that wraps",
		},
		{
			args:                []string{"doc", "gno.land/p/nt/avl/v0"},
			stdoutShouldContain: "func NewTree",
		},
		{
			args:                []string{"doc", "-u", "gno.land/p/nt/avl/v0.Node"},
			stdoutShouldContain: "node *Node",
		},
		{
			args:             []string{"doc", "dkfdkfkdfjkdfj"},
			errShouldContain: "package not found",
		},
		{
			args:             []string{"doc", "There.Are.Too.Many.Dots"},
			errShouldContain: "invalid arguments",
		},

		// Testing stdlib: testing-only package
		{
			args:                []string{"doc", "testing"},
			stdoutShouldContain: "only available in gno test",
		},
		{
			args:                []string{"doc", "testing.T"},
			stdoutShouldContain: "type T",
		},

		// Testing stdlib: testing-only package (fmt)
		{
			args:                []string{"doc", "fmt"},
			stdoutShouldContain: "only available in gno test",
		},

		// Regular stdlib unchanged (no testing stdlib counterpart)
		{
			args:                []string{"doc", "strconv"},
			stdoutShouldContain: "func Itoa",
		},
	}
	testMainCaseRun(t, tc)
}

// Start default MCP HTTP with core tools (no Eino).
//
//	go run ./examples/mcp [/path/to/project]
package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ningharness"
	"ningharness/defaults"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fatal(err)
	}
	rt, err := defaults.Open(defaults.Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(abs, ".ningharness-data"),
			Root:    abs,
		},
		WithoutEino: true,
	})
	if err != nil {
		fatal(err)
	}
	defer rt.Close()

	fmt.Println("MCP:", rt.MCPURL())
	fmt.Println("Ctrl+C to stop")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// Example: defaults (MCP core tools + Eino guest) and one user message.
//
//	export NINGHARNESS_API_KEY=...   # or OPENAI_API_KEY
//	# optional: OPENAI_BASE_URL, NINGHARNESS_MODEL
//	go run ./examples/chat [/path/to/project] ["your message"]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ningharness"
	"ningharness/defaults"
)

func main() {
	root := "."
	msg := "List the project files in one short paragraph."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if len(os.Args) > 2 {
		msg = os.Args[2]
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
		// MCPAddr: "" → listen 127.0.0.1:51020 with core tools
		// WithoutEino: false → default Eino Guest
	})
	if err != nil {
		fatal(err)
	}
	defer rt.Close()

	fmt.Println("MCP:", rt.MCPURL())
	reply, err := rt.Chat(context.Background(), msg)
	if err != nil {
		fatal(err)
	}
	fmt.Println(reply)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

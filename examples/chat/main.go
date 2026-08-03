// Complete demo: MCP core tools + Eino Guest + one user message.
//
//	1. Edit einoCfg below
//	2. go run ./examples/chat [/path/to/project] ["your message"]
//	3. Paste the printed Cursor snippet; reply prints; Ctrl+C stops MCP
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ningharness"
	"ningharness/defaults"
	"ningharness/guest/eino"
)

var einoCfg = eino.Opts{
	APIKey:  "sk-...",                    // required
	BaseURL: "https://api.openai.com/v1", // gateway / compatible endpoint
	Model:   "gpt-4o-mini",
}

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
		Eino: einoCfg,
		// MCPAddr: "" → 127.0.0.1:51020 (auto free port if busy)
		// WithoutEino: true → MCP only, no key
		// MCPAddr: "off" → Chat only, no HTTP
	})
	if err != nil {
		fatal(err)
	}
	defer rt.Close()

	url := rt.MCPURL()
	fmt.Println("MCP:", url)
	fmt.Printf(`
Cursor (~/.cursor/mcp.json):
{
  "mcpServers": {
    "ningharness": {
      "url": %q
    }
  }
}
`, url)

	reply, err := rt.Chat(context.Background(), msg)
	if err != nil {
		fatal(err)
	}
	fmt.Println()
	fmt.Println(reply)
	fmt.Println()
	fmt.Println("MCP still up — Ctrl+C to stop")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

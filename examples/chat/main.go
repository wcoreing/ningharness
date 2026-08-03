// Example: one user message via default Eino Guest (no MCP HTTP).
//
//	1. Edit einoCfg below (APIKey / BaseURL / Model)
//	2. go run ./examples/chat [/path/to/project] ["your message"]
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"ningharness"
	"ningharness/defaults"
	"ningharness/guest/eino"
)

// 直接改这里；空字符串才会回落到环境变量。
var einoCfg = eino.Opts{
	APIKey:  "sk-...",                        // 必填：模型 API key
	BaseURL: "https://api.openai.com/v1",     // 网关/兼容端点改这里
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
		MCPAddr: "off",
		Eino:    einoCfg,
	})
	if err != nil {
		fatal(err)
	}
	defer rt.Close()

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

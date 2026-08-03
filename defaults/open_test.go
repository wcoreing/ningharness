package defaults

import (
	"os"
	"path/filepath"
	"testing"

	"ningharness"
)

func TestOpenWithoutEinoAndMCP(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	rt, err := Open(Opts{
		Opts: ningharness.Opts{
			DataDir: filepath.Join(dir, "data"),
			Root:    root,
		},
		MCPAddr:     "off",
		WithoutEino: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	if rt.ToolHost == nil {
		t.Fatal("ToolHost required")
	}
	if rt.MCP != nil {
		t.Fatal("MCP should be off")
	}
	if rt.Guest != nil {
		t.Fatal("Guest should be nil")
	}
	if _, err := rt.Chat(t.Context(), "hi"); err == nil {
		t.Fatal("Chat without Guest should fail")
	}
}

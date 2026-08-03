package workspace

import "testing"

func TestRegisterResolve(t *testing.T) {
	Register("p1", "ui-1", []string{"a.md"})
	if got := Resolve("p1", []string{"a.md"}); got != "ui-1" {
		t.Fatalf("got %q", got)
	}
	if got := Resolve("p1", []string{"a.md"}); got != "" {
		t.Fatalf("expected consumed, got %q", got)
	}
	Register("p1", "agent-1", []string{"b.md", "c.md"})
	if got := Resolve("p1", []string{"b.md", "c.md"}); got != "agent-1" {
		t.Fatalf("got %q", got)
	}
}

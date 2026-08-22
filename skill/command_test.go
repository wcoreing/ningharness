package skill

import "testing"

func TestResolveLeadingCommand(t *testing.T) {
	list := []Info{
		{ID: "ssh-inspect", Name: "SSH 巡检", Description: "巡检"},
		{ID: "demo", Name: "Demo", Description: "demo skill"},
	}
	rest, ids := ResolveLeadingCommand("/ssh-inspect 看看负载", list)
	if len(ids) != 1 || ids[0] != "ssh-inspect" || rest != "看看负载" {
		t.Fatalf("got rest=%q ids=%v", rest, ids)
	}
	rest, ids = ResolveLeadingCommand("/unknown", list)
	if ids != nil || rest != "/unknown" {
		t.Fatalf("unknown should pass through: rest=%q ids=%v", rest, ids)
	}
	rest, ids = ResolveLeadingCommand("/ssh-inspect", list)
	if len(ids) != 1 || rest == "" {
		t.Fatalf("empty rest should default prompt: %q %v", rest, ids)
	}
}

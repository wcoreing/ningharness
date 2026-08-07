package guest

import (
	"context"
	"strings"
	"testing"
)

type stub struct{ got Input }

func (s *stub) Run(ctx context.Context, in Input) (string, error) {
	s.got = in
	return "ok", nil
}

func TestWireAndChat(t *testing.T) {
	wired := Wire(Input{Message: "hi", Feedforward: "## ff"})
	if !strings.Contains(wired, "hi") || !strings.Contains(wired, "## ff") {
		t.Fatalf("%q", wired)
	}
	s := &stub{}
	reply, err := Chat(context.Background(), s, "ping")
	if err != nil || reply != "ok" || s.got.Message != "ping" {
		t.Fatalf("reply=%q err=%v got=%+v", reply, err, s.got)
	}
}

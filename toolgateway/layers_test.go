package toolgateway

import "testing"

func TestAllowAskBlocksWrite(t *testing.T) {
	allow := AllowAsk()
	if allow["write_file"] || allow["edit"] || allow["enqueue_goal"] {
		t.Fatalf("ask must not allow write/control: %+v", allow)
	}
	if !allow["read_file"] || !allow["grep"] || !allow["list_lessons"] {
		t.Fatalf("ask must allow read+cognition: %+v", allow)
	}
}

func TestAllowLayersUnion(t *testing.T) {
	a := AllowLayers(LayerControl)
	if !a["list_queue"] || a["read_file"] {
		t.Fatalf("%+v", a)
	}
}

func TestCoreToolLayerCoverage(t *testing.T) {
	h := New(nil, nil)
	h.ensureCoreHandlers()
	h.mu.Lock()
	defer h.mu.Unlock()
	for name := range h.handlers {
		if CoreToolLayer(name) == "" {
			t.Fatalf("core tool %q missing layer", name)
		}
	}
}

func TestArmPurposeAsk(t *testing.T) {
	h := New(nil, nil)
	h.ArmPurpose("ask")
	if err := h.CheckToolInterceptor("write_file"); err == nil {
		t.Fatal("want blocked")
	}
	if err := h.CheckToolInterceptor("read_file"); err != nil {
		t.Fatal(err)
	}
	h.FinishTurn("ok", "")
	if err := h.CheckToolInterceptor("write_file"); err != nil {
		t.Fatalf("after FinishTurn interceptor should be clear: %v", err)
	}
}

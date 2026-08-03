package history

import (
	"strings"
	"testing"
)

func TestEncodeDecodeResourceIDs(t *testing.T) {
	raw := EncodeResourceIDs([]int64{3, 3, 0, 7})
	if raw != `[3,7]` {
		t.Fatalf("encode=%q", raw)
	}
	got := DecodeResourceIDs(raw)
	if len(got) != 2 || got[0] != 3 || got[1] != 7 {
		t.Fatalf("%v", got)
	}
}

func TestWireToolWithResourceIDs(t *testing.T) {
	got := WireTool(Msg{
		Role: "tool", Content: "ok", ResourceIDsJSON: `[42]`,
	})
	if !strings.Contains(got, "ok") || !strings.Contains(got, "recall_resource resource_id=42") {
		t.Fatalf("%q", got)
	}
}

func TestToolCallSpecResourceIDsRoundTrip(t *testing.T) {
	raw := EncodeToolCalls([]ToolCallSpec{{
		ID: "c1", Name: "get_skill", Arguments: `{"skill":"extract"}`, ResourceIDs: []int64{1918},
	}})
	if strings.Contains(raw, "〔resource#") {
		t.Fatalf("arguments polluted: %s", raw)
	}
	if !strings.Contains(raw, `"resource_ids":[1918]`) {
		t.Fatalf("missing resource_ids: %s", raw)
	}
}

func TestMergeResourceIDs(t *testing.T) {
	got := MergeResourceIDs([]int64{1, 2}, []int64{2, 3}, nil)
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("%v", got)
	}
}

func TestCollectResourceIDsStructured(t *testing.T) {
	msgs := []Msg{
		{Role: "assistant", ToolCallsJSON: EncodeToolCalls([]ToolCallSpec{{
			ID: "c1", Name: "read_file", Arguments: `{}`, ResourceIDs: []int64{1},
		}})},
		{Role: "tool", ToolCallID: "c1", Content: "short", ResourceIDsJSON: `[2,3]`},
	}
	got := CollectResourceIDs(msgs)
	if len(got) != 3 {
		t.Fatalf("%v", got)
	}
}

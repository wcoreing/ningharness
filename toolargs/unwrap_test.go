package toolargs

import (
	"encoding/json"
	"testing"
)

func TestFlattenArguments_stringEnvelope(t *testing.T) {
	in := map[string]any{
		"arguments": `{"rel_path":"提炼/a.md"}`,
	}
	out := FlattenArguments(in)
	if out["rel_path"] != "提炼/a.md" {
		t.Fatalf("got %#v", out)
	}
}

func TestFlattenArguments_objectEnvelope(t *testing.T) {
	in := map[string]any{
		"arguments": map[string]any{"rel_path": "x.md"},
	}
	out := FlattenArguments(in)
	if out["rel_path"] != "x.md" {
		t.Fatalf("got %#v", out)
	}
}

func TestFlattenArguments_keepsRealFields(t *testing.T) {
	in := map[string]any{
		"rel_path":  "a.md",
		"arguments": "ignore",
	}
	out := FlattenArguments(in)
	if out["rel_path"] != "a.md" {
		t.Fatalf("got %#v", out)
	}
	if _, ok := out["arguments"]; !ok {
		t.Fatalf("should not unwrap when other fields present")
	}
}

func TestUnwrapArgumentsJSON(t *testing.T) {
	s := UnwrapArgumentsJSON(`{"arguments":"{\"rel_path\":\"提炼/a.md\"}"}`)
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	if m["rel_path"] != "提炼/a.md" {
		t.Fatalf("got %s", s)
	}
}

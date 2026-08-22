package jsonparse

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOK(t *testing.T) {
	raw, err := Parse(`  {"n":1,"options":[]}  `)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["n"] != float64(1) {
		t.Fatalf("n=%v", m["n"])
	}
}

func TestParseEmpty(t *testing.T) {
	if _, err := Parse("  "); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("want empty error, got %v", err)
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := Parse(`{"n":1,"op`)
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "jsonparse:") {
		t.Fatalf("got %v", err)
	}
}

func TestParseTrailing(t *testing.T) {
	_, err := Parse(`{"a":1}{"b":2}`)
	if err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("got %v", err)
	}
}

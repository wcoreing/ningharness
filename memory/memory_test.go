package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ningharness/lesson"
	"ningharness/store"
)

func TestMergeFeedforward(t *testing.T) {
	if MergeFeedforward("", "a") != "a" {
		t.Fatal("empty base")
	}
	if MergeFeedforward("a", "") != "a" {
		t.Fatal("empty patch")
	}
	got := MergeFeedforward("## A", "## B")
	if got != "## A\n\n## B" {
		t.Fatalf("%q", got)
	}
}

func TestSkillIDsFromValues(t *testing.T) {
	if SkillIDsFromValues(nil) != nil {
		t.Fatal("nil")
	}
	ids := SkillIDsFromValues(map[string]any{SkillIDsValueKey: []string{" a ", "", "b"}})
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("%v", ids)
	}
}

func TestLessonAssemble(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenProject(root); err != nil {
		t.Fatal(err)
	}
	if _, err := lesson.Append(lesson.AppendInput{
		Root: root, Scope: lesson.ScopeSkill, SkillID: "demo", Body: "prefer short replies",
	}); err != nil {
		t.Fatal(err)
	}
	m := NewLesson()
	patch, err := m.Assemble(context.Background(), AssembleInput{
		Root: root, SkillIDs: []string{"demo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patch, "prefer short") {
		t.Fatalf("patch=%q", patch)
	}
}

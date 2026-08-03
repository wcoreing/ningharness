package lesson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ningharness/deskdb"
)

func TestAppendListInjectImport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := deskdb.OpenProject(root); err != nil {
		t.Fatal(err)
	}

	e, err := Append(AppendInput{
		Root: root, Scope: ScopeSkill, SkillID: "demo", Body: "prefer short replies",
		SourceTaskID: "reflect-1", ParentTaskID: "task-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.ID == "" || e.Status != StatusActive || e.Scope != ScopeSkill || e.SkillID != "demo" {
		t.Fatalf("%+v", e)
	}
	list, err := ListBySkill(root, "demo")
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	brief := InjectBrief(root, []string{"demo"}, 900)
	if !strings.Contains(brief, "prefer short") || !strings.Contains(brief, "未认账") {
		t.Fatalf("brief=%s", brief)
	}
	noMatch := InjectBrief(root, nil, 900)
	if strings.Contains(noMatch, "prefer short") {
		t.Fatalf("empty skillIDs must not inject skill body: %s", noMatch)
	}
	if err := SetStatus(root, e.ID, StatusExpired); err != nil {
		t.Fatal(err)
	}
	brief2 := InjectBrief(root, []string{"demo"}, 900)
	if strings.Contains(brief2, "prefer short") {
		t.Fatalf("expired should not inject: %s", brief2)
	}

	md := "# LESSONS\n\n## 2026-01-01 · active\n\n<!-- lesson:les_legacy01 -->\n\nold tip\n"
	n, err := ImportFromLessonsFile(root, "demo", md)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	all, err := ListBySkill(root, "demo")
	if err != nil || len(all) < 2 {
		t.Fatalf("all=%v", all)
	}
}

func TestPersonalAndProjectScope(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	root := filepath.Join(dir, "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := deskdb.OpenProject(root); err != nil {
		t.Fatal(err)
	}

	p, err := Append(AppendInput{Scope: ScopePersonal, Body: "我偏好短句"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ProjectID != PersonalProjectID || p.Scope != ScopePersonal {
		t.Fatalf("%+v", p)
	}
	if err := Ack("", p.ID); err != nil {
		t.Fatal(err)
	}
	proj, err := Append(AppendInput{Root: root, Scope: ScopeProject, Body: "本仓用 docs/ 放正文"})
	if err != nil {
		t.Fatal(err)
	}
	brief := InjectBrief(root, nil, 900)
	if !strings.Contains(brief, "我偏好短句") || !strings.Contains(brief, "docs/") {
		t.Fatalf("brief=%s", brief)
	}
	_ = proj
}

func TestParseLegacyMarkdown(t *testing.T) {
	md := "# LESSONS\n\n## 2026-01-01 · superseded\n\n<!-- lesson:abc -->\nsource_task: t1\n\nbody here\n"
	got := parseLegacyMarkdown(md)
	if len(got) != 1 || got[0].ID != "abc" || got[0].Status != StatusSuperseded || got[0].Body != "body here" {
		t.Fatalf("%+v", got)
	}
}

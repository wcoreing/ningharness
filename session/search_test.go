package session

import (
	"strings"
	"testing"
)

func TestSearchFindsUserMessage(t *testing.T) {
	root := t.TempDir()
	pid := "p"
	st := NewStore()
	_, _ = st.Snapshot(root, pid)
	_, _ = st.Append(root, pid, "main", "user", "目标改成二十万字标书", "", "")
	_, _ = st.Append(root, pid, "main", "assistant", "好的已记下", "run-1", "")
	_, _ = st.Append(root, pid, "main", "user", "写商务部分", "", "")

	out, err := st.Search(root, pid, SearchOptions{Query: "二十万字", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "二十万字") {
		t.Fatalf("want hit: %s", out)
	}
	if !strings.Contains(out, "main/user") {
		t.Fatalf("want session/role: %s", out)
	}
	if !strings.Contains(out, "fts5") && !strings.Contains(out, "like") {
		t.Fatalf("want fts/like tag: %s", out)
	}
}

func TestSearchRanksRelevant(t *testing.T) {
	root := t.TempDir()
	pid := "p"
	st := NewStore()
	_, _ = st.Snapshot(root, pid)
	_, _ = st.Append(root, pid, "main", "user", "今天天气不错去散步", "", "")
	_, _ = st.Append(root, pid, "main", "user", "标书字数目标定为二十万字，按对照矩阵写", "", "")
	_, _ = st.Append(root, pid, "main", "user", "顺便问问晚饭吃什么", "", "")

	out, err := st.Search(root, pid, SearchOptions{Query: "二十万字", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "二十万字") {
		t.Fatalf("missing relevant: %s", out)
	}
}

func TestBuildFTSQuery(t *testing.T) {
	if g := buildFTSQuery("hello world"); !strings.Contains(g, "OR") {
		t.Fatalf("%q", g)
	}
	if g := buildFTSQuery("api"); g != "api" {
		t.Fatalf("%q", g)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	root := t.TempDir()
	st := NewStore()
	_, err := st.Search(root, "p", SearchOptions{Query: "  "})
	if err == nil {
		t.Fatal("expected error")
	}
}

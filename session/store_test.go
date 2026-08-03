package session

import (
	"path/filepath"
	"testing"
)

func TestStoreCreateAppendOrch(t *testing.T) {
	root := t.TempDir()
	pid := filepath.Base(root)
	st := NewStore()
	f, err := st.Snapshot(root, pid)
	if err != nil || f.ActiveID != MainSessionID {
		t.Fatalf("snap: %#v %v", f, err)
	}
	if OrchKey(pid, "main") != "orch:"+pid+":main" {
		t.Fatal(OrchKey(pid, "main"))
	}
	f, sess, err := st.Create(root, pid, "trial")
	if err != nil || sess.ID == "" || f.ActiveID != sess.ID {
		t.Fatalf("create: %#v %#v %v", f, sess, err)
	}
	if _, err := st.Append(root, pid, "", "user", "hello", "", ""); err != nil {
		t.Fatal(err)
	}
	brief, err := st.ActiveBrief(root, pid)
	if err != nil || brief == "" {
		t.Fatal(brief, err)
	}
}

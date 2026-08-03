package tooloutcome

import "testing"

func TestLooksFailed(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"error: missing", true},
		{"Error: boom", true},
		{"install_from_library: error: skill already exists", true},
		{`{"pauseOnError":true,"note":"ok"}`, false},
		{"Successfully wrote 'a.md'（1 字）", false},
		{"fail later", false},
		{"❌ no", true},
	}
	for _, c := range cases {
		if got := LooksFailed(c.in); got != c.want {
			t.Fatalf("%q got %v want %v", c.in, got, c.want)
		}
	}
}

func TestLooksDiskOKQueued(t *testing.T) {
	if !LooksDiskOK("Successfully wrote 'a.md'（2 字） writeId=x") {
		t.Fatal("disk ok")
	}
	if !LooksDiskOK("Successfully edited 'a.md'（1 处 · 3 字）") {
		t.Fatal("edit ok")
	}
	if LooksDiskOK("error: write failed") {
		t.Fatal("error not disk")
	}
	if !LooksQueued("已入队（未落盘） job=q-1") {
		t.Fatal("queued")
	}
	if LooksDiskOK("已入队（未落盘） job=q-1") {
		t.Fatal("queued is not disk")
	}
}

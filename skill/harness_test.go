package skill

import "testing"

func TestFilterLessonInjectIDs(t *testing.T) {
	got := FilterLessonInjectIDs([]string{"writing", "skill-train", "extract", "bid-writing", ""})
	if len(got) != 2 || got[0] != "writing" || got[1] != "bid-writing" {
		t.Fatalf("%#v", got)
	}
	if FilterLessonInjectIDs(nil) != nil {
		t.Fatal("nil in")
	}
	if len(FilterLessonInjectIDs([]string{"skill-train"})) != 0 {
		t.Fatal("orchestration alone")
	}
	if len(FilterLessonInjectIDs([]string{"extract"})) != 0 {
		t.Fatal("extract orchestration")
	}
}

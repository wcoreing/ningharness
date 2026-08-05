package job

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFormatSidebarEnqueueAndProgress(t *testing.T) {
	job := Job{
		ID:        "q-b",
		TargetRel: "章节/a.md",
		StepDone:  1,
		StepTotal: 2,
		Steps: []Step{
			{Rel: "章节/a.md", Status: StepDone},
			{Rel: "章节/b.md", Status: StepPending},
		},
	}
	enq := FormatSidebarEnqueue(job)
	if !strings.Contains(enq, "已入队批写 2 节") || !strings.Contains(enq, "章节/a.md") {
		t.Fatalf("enqueue=%s", enq)
	}
	step := FormatSidebarProgress(job, ProgressStep)
	if !strings.Contains(step, "批写进度 1/2") || !strings.Contains(step, "章节/a.md") {
		t.Fatalf("step=%s", step)
	}
	job.StepDone = 2
	job.Steps[1].Status = StepDone
	done := FormatSidebarProgress(job, ProgressDone)
	if !strings.Contains(done, "批写完成 2/2") {
		t.Fatalf("done=%s", done)
	}
}

func TestFormatEnqueueOK(t *testing.T) {
	msg := FormatEnqueueOK(Job{
		ID:        "q-1",
		Status:    StatusQueued,
		TargetRel: "章节/a.md",
		StepTotal: 3,
	})
	if !strings.Contains(msg, "已入队（未落盘）") {
		t.Fatalf("msg=%s", msg)
	}
	if !strings.Contains(msg, "id=q-1") || !strings.Contains(msg, "target=`章节/a.md`") {
		t.Fatalf("msg=%s", msg)
	}
	if !strings.Contains(msg, "本轮只写") {
		t.Fatalf("want write-target hint: %s", msg)
	}
	if !strings.Contains(msg, "steps=3") || !strings.Contains(msg, "list_queue") {
		t.Fatalf("msg=%s", msg)
	}
	if strings.Contains(msg, "Successfully wrote") {
		t.Fatal("enqueue must not look like write success")
	}
	goal := FormatEnqueueOK(Job{
		ID:            "q-g",
		Type:          JobTypeGoal,
		Status:        StatusQueued,
		GoalMaxRounds: 20,
		Prompt:        "until green",
	})
	if !strings.Contains(goal, "type=goal") || !strings.Contains(goal, "maxRounds=20") || !strings.Contains(goal, "GOAL.yaml") {
		t.Fatalf("goal=%s", goal)
	}
}

func TestFormatAgentSnapshotOmitsPrompt(t *testing.T) {
	snap := Snapshot{
		Jobs: []Job{{
			ID:     "q-2",
			Status: StatusRunning,
			Prompt: strings.Repeat("长提示词正文不应出现在 jobs 全文里 ", 20),
			Steps: []Step{
				{Rel: "a.md", Title: "a", Status: StepDone},
				{Rel: "b.md", Title: "b", Status: StepRunning},
			},
			StepDone:  1,
			StepTotal: 2,
			TargetRel: "a.md",
		}},
		Stats: Stats{Running: 1},
	}
	got := FormatAgentSnapshot(snap)
	if !strings.Contains(got.Note, "入队成功≠落盘") || !strings.Contains(got.Note, "本轮只写") {
		t.Fatalf("note=%s", got.Note)
	}
	if len(got.Jobs) != 1 || got.Jobs[0].ID != "q-2" {
		t.Fatalf("%+v", got.Jobs)
	}
	if got.Jobs[0].PromptBrief == "" {
		t.Fatal("want promptBrief")
	}
	if utf8.RuneCountInString(got.Jobs[0].PromptBrief) > maxPromptBrief+1 {
		t.Fatalf("promptBrief too long: %q", got.Jobs[0].PromptBrief)
	}
	if len(got.Jobs[0].Steps) != 2 || got.Jobs[0].Steps[1].Rel != "b.md" {
		t.Fatalf("steps=%+v", got.Jobs[0].Steps)
	}
}

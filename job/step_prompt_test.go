package job

import (
	"strings"
	"testing"
)

func TestBuildStepUserVsWire(t *testing.T) {
	task := Job{
		Prompt: "收紧第一章钩子",
		Steps: []Step{
			{Rel: "章节/第一章.md", Title: "第一章.md", Status: StepPending},
		},
		StepTotal: 1,
	}
	user := BuildStepUserPrompt(task, 0)
	wire := BuildStepWirePrompt(task, 0)
	if !strings.Contains(user, "收紧第一章钩子") {
		t.Fatalf("user=%q", user)
	}
	if strings.Contains(user, "建议路径") || strings.Contains(user, "目标路径") || strings.Contains(user, "章节/第一章") {
		t.Fatalf("path must not be injected into prompt: %q", user)
	}
	if strings.Contains(user, "批任务进度") || strings.Contains(user, "LESSONS") {
		t.Fatalf("user prompt must stay human-facing: %q", user)
	}
	if wire != user {
		t.Fatalf("wire should equal user body, got wire=%q user=%q", wire, user)
	}
}

func TestWrapRetryPrefersWirePrompt(t *testing.T) {
	p := WrapRetryPrompt(Job{
		Prompt:     "人话短句",
		WirePrompt: "",
		TargetRel:  "章节/第一章.md",
		Steps:      []Step{{Rel: "章节/第一章.md", Status: StepPending}},
		StepTotal:  1,
		RetryCount: 1,
	})
	if !strings.Contains(p, "人话短句") {
		t.Fatalf("missing body: %q", p)
	}
	if !strings.Contains(p, "失败重启") {
		t.Fatalf("missing retry header: %q", p)
	}
}

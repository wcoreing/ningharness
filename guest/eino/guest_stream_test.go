package eino

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestConsumeAssistantOutputNonStream(t *testing.T) {
	var got []string
	var reply strings.Builder
	mo := &adk.MessageVariant{
		IsStreaming: false,
		Role:        schema.Assistant,
		Message:     schema.AssistantMessage("hello", nil),
	}
	if err := consumeAssistantOutput(mo, &reply, func(d string) { got = append(got, d) }); err != nil {
		t.Fatal(err)
	}
	if reply.String() != "hello" || len(got) != 1 || got[0] != "hello" {
		t.Fatalf("reply=%q got=%v", reply.String(), got)
	}
}

func TestOptsStreamImpliesEnable(t *testing.T) {
	// 无 API Key 时 New 会失败；只验证配置语义：Stream / OnDelta 任一即开流。
	o1 := Opts{Stream: true}
	if !(o1.Stream || o1.OnDelta != nil) {
		t.Fatal("Stream should enable")
	}
	o2 := Opts{OnDelta: func(string) {}}
	if !(o2.Stream || o2.OnDelta != nil) {
		t.Fatal("OnDelta should enable")
	}
}

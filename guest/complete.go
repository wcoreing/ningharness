package guest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

// CompleteInput 一轮模型完成（可选工具 + 流式）。
type CompleteInput struct {
	Model    einomodel.ToolCallingChatModel
	Messages []*schema.Message
	Tools    []*schema.ToolInfo
	// Stream 显式开流；也可由 ctx 上 WithDeltaHandler 触发。
	Stream bool
}

// Complete 调用 ChatModel；流式时把文本增量推到 DeltaHandler(ctx)。
func Complete(ctx context.Context, in CompleteInput) (*schema.Message, error) {
	if in.Model == nil {
		return nil, fmt.Errorf("guest: nil chat model")
	}
	cm := in.Model
	if len(in.Tools) > 0 {
		bound, err := cm.WithTools(in.Tools)
		if err != nil {
			return nil, err
		}
		cm = bound
	}
	stream := in.Stream || DeltaHandler(ctx) != nil
	if !stream {
		return cm.Generate(ctx, in.Messages)
	}
	sr, err := cm.Stream(ctx, in.Messages)
	if err != nil {
		return nil, err
	}
	defer sr.Close()
	onDelta := DeltaHandler(ctx)
	var chunks []*schema.Message
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		chunk, err := sr.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if onDelta != nil && chunk.Content != "" {
			onDelta(chunk.Content)
		}
	}
	if len(chunks) == 0 {
		return schema.AssistantMessage("", nil), nil
	}
	return schema.ConcatMessages(chunks)
}

// ToolInfoFromJSONSchema 用 OpenAI function parameters JSON object 构造 ToolInfo。
func ToolInfoFromJSONSchema(name, desc string, parametersJSON map[string]any) (*schema.ToolInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("guest: empty tool name")
	}
	if parametersJSON == nil {
		parametersJSON = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	raw, err := json.Marshal(parametersJSON)
	if err != nil {
		return nil, err
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("guest: tool params: %w", err)
	}
	return &schema.ToolInfo{
		Name:        name,
		Desc:        desc,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(&s),
	}, nil
}

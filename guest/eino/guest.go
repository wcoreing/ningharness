// Package eino 可选默认 Guest：Eino ReAct + Gateway.Invoke 核工具。
// 不 import 本包则不拉起 Eino；也可用自备 Guest 替换。
package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wcoreing/ningharness/guest"
	guestmodel "github.com/wcoreing/ningharness/guest/model"
	"github.com/wcoreing/ningharness/toolgateway"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

const defaultInstruction = `You are a helpful project agent. Use tools to inspect and edit files under the project root.
Prefer read_file / list_tree / grep before write_file. Keep replies short; after writes report paths only.`

// Opts Eino Guest 配置；空字段从环境变量回落（经 guest/model 工厂）。
type Opts struct {
	Provider    string // qwen|deepseek|minimax|ollama|ark|tokenhub|openai；空=openai
	APIKey      string
	BaseURL     string
	Model       string
	Thinking    bool
	Instruction string
	MaxIters    int // 默认 24
}

// New 构造默认 Eino Guest；gw 不可空；模型经 guest/model.NewChatModel。
func New(gw *toolgateway.Gateway, opts Opts) (guest.Guest, error) {
	if gw == nil {
		return nil, fmt.Errorf("eino guest: nil Gateway")
	}
	prov := guestmodel.ParseProvider(opts.Provider)
	if strings.TrimSpace(opts.Provider) == "" {
		prov = guestmodel.ProviderOpenAI
	}
	cm, modelName, err := guestmodel.NewChatModel(context.Background(), guestmodel.Config{
		Provider: prov,
		APIKey:   opts.APIKey,
		BaseURL:  opts.BaseURL,
		Model:    opts.Model,
		Thinking: opts.Thinking,
	})
	if err != nil {
		return nil, fmt.Errorf("eino guest model: %w", err)
	}
	instruction := strings.TrimSpace(opts.Instruction)
	if instruction == "" {
		instruction = defaultInstruction
	}
	maxIt := opts.MaxIters
	if maxIt <= 0 {
		maxIt = 24
	}
	tools, err := buildTools(gw)
	if err != nil {
		return nil, err
	}
	return &einoGuest{
		gw:          gw,
		cm:          cm,
		tools:       tools,
		instruction: instruction,
		maxIters:    maxIt,
		modelLabel:  modelName,
	}, nil
}

type einoGuest struct {
	gw          *toolgateway.Gateway
	cm          model.ToolCallingChatModel
	tools       []tool.BaseTool
	instruction string
	maxIters    int
	modelLabel  string
}

func (g *einoGuest) Run(ctx context.Context, in guest.Input) (string, error) {
	if g == nil {
		return "", fmt.Errorf("eino guest: nil")
	}
	message := guest.Wire(in)
	if message == "" {
		return "", fmt.Errorf("eino guest: empty message")
	}
	agentInst, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "github.com/wcoreing/ningharness",
		Description:   "ningharness default Eino guest",
		Instruction:   g.instruction,
		Model:         g.cm,
		MaxIterations: g.maxIters,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: g.tools},
		},
	})
	if err != nil {
		return "", err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agentInst, EnableStreaming: false})
	iter := runner.Run(ctx, []*schema.Message{schema.UserMessage(message)})
	var reply strings.Builder
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev == nil {
			continue
		}
		if ev.Err != nil {
			return "", ev.Err
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		mo := ev.Output.MessageOutput
		if mo.Role != schema.Assistant {
			continue
		}
		if mo.Message != nil {
			if c := strings.TrimSpace(mo.Message.Content); c != "" {
				reply.WriteString(c)
			}
		}
	}
	out := strings.TrimSpace(reply.String())
	if out == "" {
		out = "(no text reply)"
	}
	return out, nil
}

type fileArgs struct {
	RelPath string `json:"rel_path"`
}

type writeArgs struct {
	RelPath string `json:"rel_path"`
	Content string `json:"content"`
}

type grepArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

type emptyArgs struct{}

func buildTools(gw *toolgateway.Gateway) ([]tool.BaseTool, error) {
	invoke := func(ctx context.Context, name string, args any) (string, error) {
		raw, err := json.Marshal(args)
		if err != nil {
			return "", err
		}
		return gw.Invoke(ctx, name, string(raw))
	}
	readT, err := toolutils.InferTool("read_file", "Read a text file under the project root",
		func(ctx context.Context, in fileArgs) (string, error) {
			return invoke(ctx, "read_file", in)
		})
	if err != nil {
		return nil, err
	}
	listT, err := toolutils.InferTool("list_tree", "List project file tree",
		func(ctx context.Context, _ emptyArgs) (string, error) {
			return invoke(ctx, "list_tree", map[string]any{})
		})
	if err != nil {
		return nil, err
	}
	writeT, err := toolutils.InferTool("write_file", "Write full file content at rel_path",
		func(ctx context.Context, in writeArgs) (string, error) {
			return invoke(ctx, "write_file", in)
		})
	if err != nil {
		return nil, err
	}
	grepT, err := toolutils.InferTool("grep", "Search file contents in the project",
		func(ctx context.Context, in grepArgs) (string, error) {
			return invoke(ctx, "grep", in)
		})
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{readT, listT, writeT, grepT}, nil
}

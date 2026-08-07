// Package eino 可选默认 Guest：Eino ReAct + Gateway.Invoke 核工具。
// 不 import 本包则不拉起 Eino；也可用自备 Guest 替换。
package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"ningharness/guest"
	"ningharness/toolgateway"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/cloudwego/eino-ext/components/model/openai"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

const defaultInstruction = `You are a helpful project agent. Use tools to inspect and edit files under the project root.
Prefer read_file / list_tree / grep before write_file. Keep replies short; after writes report paths only.`

// Opts Eino Guest 配置；空字段从环境变量回落。
type Opts struct {
	APIKey      string // NINGHARNESS_API_KEY 或 OPENAI_API_KEY
	BaseURL     string // NINGHARNESS_BASE_URL 或 OPENAI_BASE_URL
	Model       string // NINGHARNESS_MODEL，默认 gpt-4o-mini
	Instruction string
	MaxIters    int // 默认 24
}

// New 构造默认 Eino Guest；gw 不可空。
func New(gw *toolgateway.Gateway, opts Opts) (guest.Guest, error) {
	if gw == nil {
		return nil, fmt.Errorf("eino guest: nil Gateway")
	}
	apiKey := strings.TrimSpace(opts.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("NINGHARNESS_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if apiKey == "" {
		return nil, fmt.Errorf("eino guest: set NINGHARNESS_API_KEY or OPENAI_API_KEY")
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("NINGHARNESS_BASE_URL"))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	}
	modelName := strings.TrimSpace(opts.Model)
	if modelName == "" {
		modelName = strings.TrimSpace(os.Getenv("NINGHARNESS_MODEL"))
	}
	if modelName == "" {
		modelName = "gpt-4o-mini"
	}
	instruction := strings.TrimSpace(opts.Instruction)
	if instruction == "" {
		instruction = defaultInstruction
	}
	maxIt := opts.MaxIters
	if maxIt <= 0 {
		maxIt = 24
	}

	cfg := &openai.ChatModelConfig{
		APIKey: apiKey,
		Model:  modelName,
	}
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	cm, err := openai.NewChatModel(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("eino guest model: %w", err)
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
	cm          *openai.ChatModel
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
		Name:          "ningharness",
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

package model

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	arkmodel "github.com/cloudwego/eino-ext/components/model/ark"
	deepseekmodel "github.com/cloudwego/eino-ext/components/model/deepseek"
	ollamamodel "github.com/cloudwego/eino-ext/components/model/ollama"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	qwenmodel "github.com/cloudwego/eino-ext/components/model/qwen"
	volcark "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// NewChatModel 按 Provider 构造 Eino ToolCallingChatModel。
func NewChatModel(ctx context.Context, cfg Config) (model.ToolCallingChatModel, string, error) {
	cfg.Provider = ParseProvider(string(cfg.Provider))
	httpClient := &http.Client{Timeout: 15 * time.Minute}
	switch cfg.Provider {
	case ProviderDeepSeek:
		return newDeepSeek(ctx, cfg, httpClient)
	case ProviderMiniMax:
		return newMiniMax(ctx, cfg, httpClient)
	case ProviderOllama:
		return newOllama(ctx, cfg, httpClient)
	case ProviderArk:
		return newArk(ctx, cfg, httpClient)
	case ProviderTokenHub:
		return newTokenHub(ctx, cfg, httpClient)
	case ProviderOpenAI:
		return newOpenAICompatible(ctx, cfg, httpClient)
	default:
		return newQwen(ctx, cfg, httpClient)
	}
}

func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func newOpenAICompatible(_ context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = envFirst("NINGHARNESS_API_KEY", "OPENAI_API_KEY", "LLM_API_KEY")
	}
	if apiKey == "" {
		return nil, "", fmt.Errorf("未配置 OpenAI 兼容 API Key")
	}
	modelName := ResolveModel(ProviderOpenAI, cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "OPENAI_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		modelName = openAIModelDefault
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = envFirst("NINGHARNESS_BASE_URL", "OPENAI_BASE_URL", "LLM_BASE_URL")
	}
	conf := &openaimodel.ChatModelConfig{
		APIKey:     apiKey,
		Model:      modelName,
		HTTPClient: httpClient,
	}
	if base != "" {
		conf.BaseURL = base
	}
	cm, err := openaimodel.NewChatModel(context.Background(), conf)
	if err != nil {
		return nil, "", err
	}
	return cm, modelName, nil
}

func newMiniMax(_ context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = envFirst("MINIMAX_API_KEY", "LLM_API_KEY")
	}
	if apiKey == "" {
		return nil, "", fmt.Errorf("未配置 MiniMax API Key")
	}
	modelName := ResolveModel(ProviderMiniMax, cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		modelName = miniMaxModel
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = envFirst("LLM_BASE_URL")
	}
	if base == "" {
		base = miniMaxBaseURL
	}
	conf := &openaimodel.ChatModelConfig{
		APIKey:     apiKey,
		BaseURL:    base,
		Model:      modelName,
		HTTPClient: httpClient,
	}
	thinkType := "disabled"
	if cfg.Thinking {
		thinkType = "adaptive"
	}
	conf.ExtraFields = map[string]any{
		"thinking": map[string]any{"type": thinkType},
	}
	cm, err := openaimodel.NewChatModel(context.Background(), conf)
	if err != nil {
		return nil, "", err
	}
	return cm, modelName, nil
}

func newQwen(_ context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = envFirst("DASHSCOPE_API_KEY")
	}
	if apiKey == "" {
		return nil, "", fmt.Errorf("未配置百炼 API Key")
	}
	modelName := ResolveModel(ProviderQwen, cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		modelName = dashScopeModel
	}
	enableThinking := cfg.Thinking
	qBase := strings.TrimSpace(cfg.BaseURL)
	if qBase == "" {
		qBase = envFirst("LLM_BASE_URL")
	}
	if qBase == "" {
		qBase = dashScopeBaseURL
	}
	cm, err := qwenmodel.NewChatModel(context.Background(), &qwenmodel.ChatModelConfig{
		APIKey:         apiKey,
		BaseURL:        qBase,
		Model:          modelName,
		HTTPClient:     httpClient,
		EnableThinking: &enableThinking,
	})
	if err != nil {
		return nil, "", err
	}
	return cm, modelName, nil
}

func newDeepSeek(_ context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = envFirst("DEEPSEEK_API_KEY")
	}
	if apiKey == "" {
		return nil, "", fmt.Errorf("未配置 DeepSeek API Key")
	}
	modelName := ResolveModel(ProviderDeepSeek, cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		modelName = deepSeekModel
	}
	thinkingType := "disabled"
	if cfg.Thinking {
		thinkingType = "enabled"
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		base = envFirst("LLM_BASE_URL", "WNAI_BASE_URL", "NINGHARNESS_BASE_URL")
	}
	if base == "" {
		base = deepSeekBaseURL
	}
	cm, err := deepseekmodel.NewChatModel(context.Background(), &deepseekmodel.ChatModelConfig{
		APIKey:     apiKey,
		BaseURL:    base,
		Model:      modelName,
		HTTPClient: httpClient,
		ThinkingConfig: &deepseekmodel.ThinkingConfig{
			Type: thinkingType,
		},
	})
	if err != nil {
		return nil, "", err
	}
	return cm, modelName, nil
}

func newOllama(ctx context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	modelName := strings.TrimSpace(cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		return nil, "", fmt.Errorf("未配置 Ollama 模型名（如 qwen2.5:7b）")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(envFirst("OLLAMA_BASE_URL"), "/")
	}
	if baseURL == "" {
		baseURL = strings.TrimRight(ollamaBaseURLDefault, "/v1")
	}
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	resolved, err := resolveOllamaModel(ctx, baseURL+"/v1", modelName)
	if err != nil {
		return nil, "", err
	}
	conf := &ollamamodel.ChatModelConfig{
		BaseURL:    baseURL,
		Model:      resolved,
		HTTPClient: httpClient,
	}
	if cfg.Thinking {
		thinking := ollamamodel.ThinkValue{Value: true}
		conf.Thinking = &thinking
	}
	cm, err := ollamamodel.NewChatModel(ctx, conf)
	if err != nil {
		return nil, "", err
	}
	return cm, resolved, nil
}

func newArk(_ context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = envFirst("ARK_API_KEY", "VOLCENGINE_API_KEY")
	}
	if apiKey == "" {
		return nil, "", fmt.Errorf("未配置火山引擎 Ark API Key")
	}
	modelName := ResolveModel(ProviderArk, cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		modelName = arkModel
	}
	thinkingType := volcark.ThinkingTypeDisabled
	if cfg.Thinking {
		thinkingType = volcark.ThinkingTypeEnabled
	}
	cm, err := arkmodel.NewChatModel(context.Background(), &arkmodel.ChatModelConfig{
		APIKey:     apiKey,
		Model:      modelName,
		HTTPClient: httpClient,
		Thinking:   &volcark.Thinking{Type: thinkingType},
	})
	if err != nil {
		return nil, "", err
	}
	return cm, modelName, nil
}

func newTokenHub(_ context.Context, cfg Config, httpClient *http.Client) (model.ToolCallingChatModel, string, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		apiKey = envFirst("TOKENHUB_API_KEY", "TENCENT_TOKENHUB_API_KEY")
	}
	if apiKey == "" {
		return nil, "", fmt.Errorf("未配置 TokenHub API Key")
	}
	modelName := ResolveModel(ProviderTokenHub, cfg.Model)
	if modelName == "" {
		modelName = envFirst("NINGHARNESS_MODEL", "WNAI_MODEL")
	}
	if modelName == "" {
		modelName = tokenHubModel
	}
	conf := &openaimodel.ChatModelConfig{
		APIKey:     apiKey,
		BaseURL:    tokenHubBaseURL,
		Model:      modelName,
		HTTPClient: httpClient,
	}
	if cfg.Thinking {
		conf.ReasoningEffort = openaimodel.ReasoningEffortLevelMedium
	}
	cm, err := openaimodel.NewChatModel(context.Background(), conf)
	if err != nil {
		return nil, "", err
	}
	return cm, modelName, nil
}

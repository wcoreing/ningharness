package model

import "strings"

// Provider 大模型提供方。
type Provider string

const (
	ProviderQwen     Provider = "qwen"
	ProviderDeepSeek Provider = "deepseek"
	ProviderMiniMax  Provider = "minimax"
	ProviderOllama   Provider = "ollama"
	ProviderArk      Provider = "ark"
	ProviderTokenHub Provider = "tokenhub"
	ProviderOpenAI   Provider = "openai" // OpenAI 兼容（含自定义 BaseURL）
)

// Config 构造 ChatModel 的参数。
type Config struct {
	Provider Provider
	APIKey   string
	BaseURL  string
	Model    string
	// Thinking：DeepSeek / 千问 / Ark / Ollama 等思考开关；MiniMax→adaptive|disabled。
	Thinking bool
}

// ParseProvider 从设置字符串解析后端。
func ParseProvider(raw string) Provider {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "deepseek", "deepseek-reasoner", "deepseek-v4-pro", "deepseek-v4-flash":
		return ProviderDeepSeek
	case "minimax", "mini":
		return ProviderMiniMax
	case "ollama":
		return ProviderOllama
	case "ark", "volcengine", "volc", "doubao", "火山", "火山引擎":
		return ProviderArk
	case "tokenhub", "tencent", "tencentmaas", "token hub", "hunyuan", "混元":
		return ProviderTokenHub
	case "openai", "gpt", "compatible":
		return ProviderOpenAI
	case "qwen", "dashscope", "tongyi":
		return ProviderQwen
	default:
		if strings.TrimSpace(raw) == "" {
			return ProviderOpenAI
		}
		return ProviderQwen
	}
}

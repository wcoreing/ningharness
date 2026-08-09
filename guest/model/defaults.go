package model

import "strings"

const (
	dashScopeBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	dashScopeModel   = "qwen-plus-latest"

	deepSeekBaseURL = "https://api.deepseek.com"
	deepSeekModel   = "deepseek-v4-flash"

	miniMaxBaseURL = "https://api.minimaxi.com/v1"
	miniMaxModel   = "MiniMax-M3"

	arkModel = "doubao-seed-1-6-250615"

	tokenHubBaseURL = "https://tokenhub.tencentmaas.com/v1"
	tokenHubModel   = "deepseek-v3"

	ollamaBaseURLDefault = "http://127.0.0.1:11434/v1"

	openAIModelDefault = "gpt-4o-mini"
)

// DefaultModel 后端内置默认模型名（Ollama 无统一默认，返回空）。
func DefaultModel(p Provider) string {
	switch ParseProvider(string(p)) {
	case ProviderDeepSeek:
		return deepSeekModel
	case ProviderMiniMax:
		return miniMaxModel
	case ProviderArk:
		return arkModel
	case ProviderTokenHub:
		return tokenHubModel
	case ProviderOpenAI:
		return openAIModelDefault
	case ProviderOllama:
		return ""
	default:
		return dashScopeModel
	}
}

// DefaultBaseURL 后端默认 BaseURL（Ark 走 SDK 默认，返回空）。
func DefaultBaseURL(p Provider) string {
	switch ParseProvider(string(p)) {
	case ProviderDeepSeek:
		return deepSeekBaseURL
	case ProviderMiniMax:
		return miniMaxBaseURL
	case ProviderTokenHub:
		return tokenHubBaseURL
	case ProviderQwen:
		return dashScopeBaseURL
	case ProviderOllama:
		return ollamaBaseURLDefault
	default:
		return ""
	}
}

// ResolveModel 空模型名时回落到后端默认。
func ResolveModel(p Provider, model string) string {
	if m := strings.TrimSpace(model); m != "" {
		return m
	}
	return DefaultModel(p)
}

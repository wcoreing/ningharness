package model

import "testing"

func TestParseProvider(t *testing.T) {
	if ParseProvider("deepseek") != ProviderDeepSeek {
		t.Fatal("deepseek")
	}
	if ParseProvider("") != ProviderOpenAI {
		t.Fatal("empty -> openai")
	}
	if ResolveModel(ProviderQwen, "") != dashScopeModel {
		t.Fatal("qwen default")
	}
}

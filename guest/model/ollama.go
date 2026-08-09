package model

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ollamaTagsResp struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// ListOllamaModels 拉取本机 Ollama 已安装模型名。
func ListOllamaModels(baseURL string) ([]string, error) {
	root := ollamaRootURL(baseURL)
	if root == "" {
		root = ollamaRootURL(ollamaBaseURLDefault)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tags, err := fetchOllamaTags(ctx, root)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func ollamaRootURL(openAIBase string) string {
	base := strings.TrimRight(strings.TrimSpace(openAIBase), "/")
	if strings.HasSuffix(base, "/v1") {
		return strings.TrimSuffix(base, "/v1")
	}
	return base
}

func fetchOllamaTags(ctx context.Context, root string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, root+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接 Ollama 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("Ollama %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var body ollamaTagsResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(body.Models))
	for _, m := range body.Models {
		if name := strings.TrimSpace(m.Name); name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// ResolveOllamaModel 将用户输入解析为 Ollama 完整模型名（支持 qwen3.6 → qwen3.6:35b）。
func ResolveOllamaModel(ctx context.Context, baseURL, name string) (string, error) {
	return resolveOllamaModel(ctx, baseURL, name)
}

func resolveOllamaModel(ctx context.Context, baseURL, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("未配置 Ollama 模型名")
	}
	root := ollamaRootURL(baseURL)
	tags, err := fetchOllamaTags(ctx, root)
	if err != nil {
		return name, nil
	}
	return resolveOllamaModelFromTags(tags, name)
}

func resolveOllamaModelFromTags(tags []string, name string) (string, error) {
	for _, m := range tags {
		if m == name {
			return name, nil
		}
	}
	var prefixMatches []string
	for _, m := range tags {
		if strings.HasPrefix(m, name+":") {
			prefixMatches = append(prefixMatches, m)
		}
	}
	switch len(prefixMatches) {
	case 1:
		return prefixMatches[0], nil
	case 0:
		return "", fmt.Errorf("Ollama 未找到模型 %q，请运行 ollama list 查看完整名称（如 qwen3.6:35b）", name)
	default:
		return "", fmt.Errorf("模型 %q 对应多个版本，请指定完整名称：%s", name, strings.Join(prefixMatches, "、"))
	}
}

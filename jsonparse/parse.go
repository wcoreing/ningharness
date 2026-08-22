// Package jsonparse 提供无业务语义的 JSON 解析（不修复、不校验 schema）。
package jsonparse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Parse 将 trim 后的文本解析为单个 JSON 值，返回规范化的 JSON 字节。
// 失败时返回标准库 decode 错误（前缀 jsonparse:）。
func Parse(text string) (json.RawMessage, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("jsonparse: empty input")
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("jsonparse: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("jsonparse: trailing data after JSON value")
		}
		return nil, fmt.Errorf("jsonparse: %w", err)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("jsonparse: marshal: %w", err)
	}
	return json.RawMessage(raw), nil
}

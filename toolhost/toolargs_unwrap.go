package toolhost

import (
	"encoding/json"
	"strings"
)

// FlattenArguments 解开模型误包的 {"arguments": {...}} / {"arguments":"{...}"}。
// 仅当除 arguments（及偶发 name/type）外没有其它业务字段时才拆，避免误伤真有 arguments 的工具。
func FlattenArguments(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	raw, ok := args["arguments"]
	if !ok {
		return args
	}
	for k := range args {
		if k == "arguments" || k == "name" || k == "type" {
			continue
		}
		return args
	}
	switch v := raw.(type) {
	case map[string]any:
		return v
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return args
		}
		var inner map[string]any
		if err := json.Unmarshal([]byte(s), &inner); err != nil || inner == nil {
			return args
		}
		return inner
	default:
		return args
	}
}

// UnwrapArgumentsJSON 对工具参数 JSON 字符串做同上解包；非 object 原样返回。
func UnwrapArgumentsJSON(argsJSON string) string {
	s := strings.TrimSpace(argsJSON)
	if s == "" || s == "null" || !strings.HasPrefix(s, "{") {
		return argsJSON
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return argsJSON
	}
	flat := FlattenArguments(m)
	if len(flat) == len(m) {
		// 可能未拆，或拆后键数碰巧相同——用指针相等判断不够；比 JSON
		if _, ok := m["arguments"]; !ok {
			return argsJSON
		}
		// 若仍含顶层 arguments 且未拆成功
		if _, still := flat["arguments"]; still {
			return argsJSON
		}
	}
	b, err := json.Marshal(flat)
	if err != nil {
		return argsJSON
	}
	return string(b)
}

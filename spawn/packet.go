// Package spawn 子代理委派：显式 Packet 传递上下文；独立 Session；Registry 管理生命周期。
// 默认不共享父对话（对齐 Hermes/Claude）；fork 日后可选。
package spawn

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	// RoleLeaf 叶子：默认禁再 spawn。
	RoleLeaf = "leaf"
	// RoleOrchestrator 可再 spawn（受 MaxDepth 约束）。
	RoleOrchestrator = "orchestrator"

	// DefaultMaxDepth 父 depth=0；子=1；默认禁止孙代。
	DefaultMaxDepth = 1

	// PurposeLeaf / PurposeOrch 写入 Job.Purpose，供 Gateway ArmPurpose。
	PurposeLeaf = "spawn_leaf"
	PurposeOrch = "spawn_orch"

	maxContextRunes = 8000
	sessionPrefix   = "spawn:"
)

// Packet 父→子显式包裹（非全历史）。
type Packet struct {
	Goal           string
	Context        string
	Artifacts      []string // 相对项目根的必读路径
	ParentSession  string
	ParentTask     string
	ParentJob      string
	ParentDepth    int    // 父当前深度；子 = ParentDepth+1
	Role           string // leaf|orchestrator；空=leaf
	Title          string
	MaxDepth       int // <1 → DefaultMaxDepth
}

// Brief 子→父回传（进父 tool_result，非 transcript）。
type Brief struct {
	SpawnID   string   `json:"spawnId"`
	Status    string   `json:"status"` // done|error|cancelled|running|queued
	Summary   string   `json:"summary,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
	Error     string   `json:"error,omitempty"`
	SessionKey string  `json:"sessionKey,omitempty"`
}

// SessionKeyFor 子代理会话键。
func SessionKeyFor(spawnID string) string {
	id := strings.TrimSpace(spawnID)
	if id == "" {
		return ""
	}
	if strings.HasPrefix(id, sessionPrefix) {
		return id
	}
	return sessionPrefix + id
}

// IDFromSession 从 spawn:… 会话解析 id；非 spawn 会话返回空。
func IDFromSession(sessionKey string) string {
	sk := strings.TrimSpace(sessionKey)
	if !strings.HasPrefix(sk, sessionPrefix) {
		return ""
	}
	return strings.TrimPrefix(sk, sessionPrefix)
}

// IsSpawnSession 是否子代理会话。
func IsSpawnSession(sessionKey string) bool {
	return IDFromSession(sessionKey) != ""
}

// ChildDepth 子深度。
func (p Packet) ChildDepth() int {
	d := p.ParentDepth + 1
	if d < 1 {
		d = 1
	}
	return d
}

// MaxDepthOrDefault 深度上限。
func (p Packet) MaxDepthOrDefault() int {
	if p.MaxDepth < 1 {
		return DefaultMaxDepth
	}
	return p.MaxDepth
}

// NormalizeRole 规范化角色。
func (p Packet) NormalizeRole() string {
	switch strings.ToLower(strings.TrimSpace(p.Role)) {
	case RoleOrchestrator, "orch":
		return RoleOrchestrator
	default:
		return RoleLeaf
	}
}

// Purpose 子 Job Purpose。
func (p Packet) Purpose() string {
	if p.NormalizeRole() == RoleOrchestrator && p.ChildDepth() < p.MaxDepthOrDefault() {
		return PurposeOrch
	}
	return PurposeLeaf
}

// Validate 入队前校验。
func (p Packet) Validate() error {
	if strings.TrimSpace(p.Goal) == "" {
		return fmt.Errorf("spawn: empty goal")
	}
	if p.ChildDepth() > p.MaxDepthOrDefault() {
		return fmt.Errorf("spawn: depth %d exceeds max %d", p.ChildDepth(), p.MaxDepthOrDefault())
	}
	return nil
}

// BuildFeedforward 子 assemble 前馈（有界）；不含父 transcript。
func BuildFeedforward(p Packet) string {
	var b strings.Builder
	b.WriteString("[spawn_packet]\n")
	b.WriteString("You are a subagent with a fresh session. You do not see the parent conversation.\n")
	b.WriteString("Complete the goal; end with a short structured summary (what you did, findings, files touched, blockers).\n")
	if ps := strings.TrimSpace(p.ParentSession); ps != "" {
		fmt.Fprintf(&b, "parent_session: %s\n", ps)
	}
	if pt := strings.TrimSpace(p.ParentTask); pt != "" {
		fmt.Fprintf(&b, "parent_task: %s\n", pt)
	}
	if ctx := strings.TrimSpace(p.Context); ctx != "" {
		b.WriteString("\n## Context\n")
		b.WriteString(trimRunes(ctx, maxContextRunes))
		b.WriteByte('\n')
	}
	arts := cleanArtifacts(p.Artifacts)
	if len(arts) > 0 {
		b.WriteString("\n## Artifacts (read these paths in the workspace)\n")
		for _, a := range arts {
			fmt.Fprintf(&b, "- `%s`\n", a)
		}
	}
	b.WriteString("[/spawn_packet]")
	return b.String()
}

// BuildPrompt 子轮用户正文 = goal。
func BuildPrompt(p Packet) string {
	return strings.TrimSpace(p.Goal)
}

func cleanArtifacts(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, a := range in {
		a = strings.TrimSpace(strings.ReplaceAll(a, "\\", "/"))
		a = strings.TrimPrefix(a, "./")
		if a == "" || strings.HasPrefix(a, "/") || strings.Contains(a, "..") {
			continue
		}
		a = filepath.ToSlash(a)
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

func trimRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

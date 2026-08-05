// Package contextpatch 台面变更补丁：Go 确定性回执（summary + 小 delta + refs）。
// 开场 feedforward 不热更新；同轮靠本补丁；大内容只挂引用，按 how 用现有工具拉。
package contextpatch

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	HowListPins   = "list_pins"
	HowReadFile   = "read_file"
	HowGetProject = "get_project"

	KindPinAdded   = "pin.added"
	KindPinRemoved = "pin.removed"
	KindPinNote    = "pin.note"
	KindFileWrote  = "file.wrote"
	KindFileEdited = "file.edited"

	maxNoteDeltaRunes = 240
)

// Ref 大内容引用：事件里不塞全文。
type Ref struct {
	Kind string `json:"kind"` // pin | file
	ID   string `json:"id"`   // 相对路径
	How  string `json:"how"`  // list_pins | read_file | get_project
}

// Patch 台面变更。
type Patch struct {
	AtMs    int64  `json:"atMs"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	Delta   string `json:"delta,omitempty"`
	Refs    []Ref  `json:"refs,omitempty"`
}

// Format 给人/模型看的回执正文（可接在原有一行成功文案后）。
func (p Patch) Format() string {
	if p.AtMs == 0 {
		p.AtMs = time.Now().UnixMilli()
	}
	var b strings.Builder
	b.WriteString("## ContextPatch\n")
	fmt.Fprintf(&b, "- **kind**：`%s`\n", strings.TrimSpace(p.Kind))
	fmt.Fprintf(&b, "- **summary**：%s\n", strings.TrimSpace(p.Summary))
	if d := strings.TrimSpace(p.Delta); d != "" {
		b.WriteString("- **delta**：\n")
		for _, line := range strings.Split(d, "\n") {
			if strings.TrimSpace(line) == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if len(p.Refs) > 0 {
		b.WriteString("- **refs**（大内容按 how 拉，勿臆造）：\n")
		for _, r := range p.Refs {
			fmt.Fprintf(&b, "  - `%s` id=`%s` how=`%s`\n", r.Kind, r.ID, r.How)
		}
	}
	b.WriteString("- **提示**：本轮开场前馈不因此改写；下一 Turn / get_project 会带上最新台面。同轮采信本 Patch + refs。\n")
	return strings.TrimSpace(b.String())
}

// Append 原回执 + Patch（空 patch 则原样返回）。
func Append(base string, p Patch) string {
	base = strings.TrimSpace(base)
	body := p.Format()
	if base == "" {
		return body
	}
	if strings.TrimSpace(p.Kind) == "" && strings.TrimSpace(p.Summary) == "" {
		return base
	}
	return base + "\n\n" + body
}

func truncate(s string, max int) (out string, truncated bool) {
	s = strings.TrimSpace(s)
	if max < 1 || utf8.RuneCountInString(s) <= max {
		return s, false
	}
	r := []rune(s)
	return string(r[:max]) + "…", true
}

// PinAdded pin_path 成功。
func PinAdded(rel, note string, total int) Patch {
	rel = strings.TrimSpace(rel)
	note = strings.TrimSpace(note)
	sum := fmt.Sprintf("钉住 `%s`（参考资料共 %d）", rel, total)
	var delta strings.Builder
	fmt.Fprintf(&delta, "- `%s`", rel)
	refs := []Ref{{Kind: "pin", ID: rel, How: HowListPins}}
	if note != "" {
		short, cut := truncate(note, maxNoteDeltaRunes)
		fmt.Fprintf(&delta, "\n  - 参考说明：%s", short)
		if cut {
			delta.WriteString("\n  - （说明已截断 · 全文 how=list_pins / get_project）")
			refs = append(refs, Ref{Kind: "pin", ID: rel, How: HowGetProject})
		}
	} else {
		delta.WriteString("\n  - （尚无参考说明 · 可 set_pin_note）")
	}
	return Patch{
		AtMs:    time.Now().UnixMilli(),
		Kind:    KindPinAdded,
		Summary: sum,
		Delta:   delta.String(),
		Refs:    refs,
	}
}

// PinRemoved unpin。
func PinRemoved(rel string, left int) Patch {
	rel = strings.TrimSpace(rel)
	return Patch{
		AtMs:    time.Now().UnixMilli(),
		Kind:    KindPinRemoved,
		Summary: fmt.Sprintf("取消钉住 `%s`（剩余 %d）", rel, left),
		Delta:   fmt.Sprintf("- 移除参考资料 `%s`", rel),
		Refs:    []Ref{{Kind: "pin", ID: rel, How: HowListPins}},
	}
}

// PinNoteSet set_pin_note。
func PinNoteSet(rel, note string, total int) Patch {
	rel = strings.TrimSpace(rel)
	note = strings.TrimSpace(note)
	if note == "" {
		return Patch{
			AtMs:    time.Now().UnixMilli(),
			Kind:    KindPinNote,
			Summary: fmt.Sprintf("清空 `%s` 的参考说明（共 %d）", rel, total),
			Delta:   fmt.Sprintf("- `%s` · 前馈改回摘录/路径", rel),
			Refs:    []Ref{{Kind: "pin", ID: rel, How: HowListPins}},
		}
	}
	short, cut := truncate(note, maxNoteDeltaRunes)
	delta := fmt.Sprintf("- `%s`\n  - 参考说明：%s", rel, short)
	refs := []Ref{{Kind: "pin", ID: rel, How: HowListPins}}
	if cut {
		delta += "\n  - （说明已截断 · 全文 how=list_pins）"
	}
	return Patch{
		AtMs:    time.Now().UnixMilli(),
		Kind:    KindPinNote,
		Summary: fmt.Sprintf("更新 `%s` 参考说明（共 %d）", rel, total),
		Delta:   delta,
		Refs:    refs,
	}
}

// FileWrote write_file 成功（正文不进 delta）。
func FileWrote(rel string, runes int, writeID string) Patch {
	rel = strings.TrimSpace(rel)
	sum := fmt.Sprintf("已落盘 `%s`（%d 字）", rel, runes)
	if w := strings.TrimSpace(writeID); w != "" {
		sum += " writeId=" + w
	}
	return Patch{
		AtMs:    time.Now().UnixMilli(),
		Kind:    KindFileWrote,
		Summary: sum,
		Delta:   fmt.Sprintf("- 文件 `%s` · %d 字（正文不在此 · how=read_file）", rel, runes),
		Refs:    []Ref{{Kind: "file", ID: rel, How: HowReadFile}},
	}
}

// FileEdited edit 成功。
func FileEdited(rel string, runes int, writeID string) Patch {
	p := FileWrote(rel, runes, writeID)
	p.Kind = KindFileEdited
	p.Summary = strings.Replace(p.Summary, "已落盘", "已编辑落盘", 1)
	return p
}

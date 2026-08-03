package job

import (
	"fmt"
	"strings"
)

// WrapRetryPrompt 失败重启信封：保留原任务意图，明确告知这是重试，先查磁盘进度再续写。
func WrapRetryPrompt(t Job) string {
	var b strings.Builder
	b.WriteString("## 失败重启\n")
	b.WriteString("这是一次**失败重启**（同一队列任务重试，非新任务）。")
	b.WriteString("磁盘上可能已有上次部分写入；**先 `list_queue` + read_file 查清现状再动手**，勿假设空白、勿整章重写已达标段落。\n")
	b.WriteString("\n建议步骤：\n")
	b.WriteString("1. `list_queue` 对齐批任务进度（batch-write Skill）\n")
	if rel := strings.TrimSpace(t.TargetRel); rel != "" {
		b.WriteString("2. `read_file` 读 `")
		b.WriteString(rel)
		b.WriteString("`（及邻章/查阅若需要），核对已有正文与结构\n")
	} else {
		b.WriteString("2. `get_project` / `list_tree` / `read_file` 对照焦点，核对磁盘已有内容\n")
	}
	b.WriteString("3. 在已有稿上续写、修补或补全；重复标题/编号会造成结构冲突\n")
	b.WriteString("4. 若本轮要改文件：调 `write_file` 后以工具结果为准再说已落盘；大段正文推荐纯文本参数（首行路径、空行、全文）\n")
	if len(t.Steps) > 0 {
		done := 0
		for _, s := range t.Steps {
			if s.Status == StepDone {
				done++
			}
		}
		fmt.Fprintf(&b, "5. 批任务进度：已完成 %d/%d 节\n", done, len(t.Steps))
	}
	if err := strings.TrimSpace(t.LastError); err != "" {
		b.WriteString("\n上次失败原因：")
		b.WriteString(trimReason(err, 240))
		b.WriteString("\n")
	}
	if t.RetryCount > 0 {
		fmt.Fprintf(&b, "重试轮次：第 %d 次\n", t.RetryCount)
	}
	b.WriteString("\n## 原任务\n")
	body := strings.TrimSpace(t.WirePrompt)
	if body == "" {
		body = strings.TrimSpace(t.Prompt)
	}
	b.WriteString(body)
	return b.String()
}

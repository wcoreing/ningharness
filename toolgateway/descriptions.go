package toolgateway

import (
	"fmt"
	"strings"

	"ningharness/contextpatch"
	"ningharness/workspace"
)

const WriteFileToolDesc = "整文件写入项目相对路径。" +
	"**新章/空盘首稿**：content=全文（常见开篇约 600～1200 字，或遵守钉住字数约定）。" +
	"**扩写/续写/大段重写**：先 read_file，再本工具覆写全文。" +
	"**定点小改**（改一句/一段）：优先用 edit。" +
	"成功返回含字数与 writeId 后才可说「已落盘」；气泡短确认，正文在文件。" +
	"大段可用纯文本：首行路径、空行、全文。"

const GrepToolDesc = "项目内内容搜索（对齐 ripgrep 心智）。" +
	"默认字面量匹配；regex=true 时用 Go 正则。" +
	"可限 path（文件/目录）与 glob（如 *.md）。" +
	"找路径/人名/设定关键词用本工具；语义找章用 semantic_recall；对话用 search_session。"

const EditToolDesc = "局部改文件：把 old_string 精确替换为 new_string 后落盘。" +
	"默认 old_string 必须在文件中唯一；多处时扩大上下文或 replace_all=true。" +
	"小改/定点修订用本工具；整章重写/空盘首稿仍用 write_file。" +
	"成功回执含字数与 writeId 后才可说已落盘。"

const RenamePathToolDesc = "同级改名。rel_path=原路径；new_name=新文件名（仅文件名，勿写 章节/…；误写完整路径会自动取 basename）。跨目录用 move_path。"

func FormatWriteOK(rel, content, writeID string) string {
	return formatWriteOK(rel, content, writeID)
}

func formatWriteOK(rel, content, writeID string) string {
	n := workspace.Count(content)
	msg := fmt.Sprintf("Successfully wrote '%s'（%d 字）", rel, n)
	if wid := strings.TrimSpace(writeID); wid != "" {
		msg += " writeId=" + wid
	}
	return contextpatch.Append(msg, contextpatch.FileWrote(rel, n, writeID))
}

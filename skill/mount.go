package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MountBrief 显式挂载 skill 时贡献前馈提示（不灌 SKILL.md 正文；Agent 须 get_skill / read_file）。
func MountBrief(projectRoot string, ids []string) string {
	root := strings.TrimSpace(projectRoot)
	ids = trimIDs(ids)
	if root == "" || len(ids) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## 本轮挂载技能\n")
	for _, id := range ids {
		fmt.Fprintf(&b, "- `%s`：首轮 **get_skill %s** 加载流程", id, id)
		if rel := scriptsReadHint(root, id); rel != "" {
			fmt.Fprintf(&b, "；含 scripts → **read_file** `%s`", rel)
		}
		b.WriteString("；禁止臆造命令。\n")
	}
	return strings.TrimSpace(b.String())
}

func scriptsReadHint(projectRoot, id string) string {
	dir := filepath.Join(SkillAbsDir(projectRoot, id), "scripts")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range ents {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		return filepath.ToSlash(filepath.Join(RootDir, id, "scripts", e.Name()))
	}
	return ""
}

func trimIDs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

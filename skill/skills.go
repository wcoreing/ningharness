// Package skill 项目级 Skill 磁盘契约与 Slot 插槽：system/skills/<id>/SKILL.md。
// 经验 SSOT 在 Store lesson_entry（库导出载体仍可写 LESSONS.md）。不含内置 packs 正文。
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	RootDir       = "system/skills"
	LegacyRootDir = "skills" // 旧布局；打开项目时 MigrateRootIfNeeded
	SkillFile     = "SKILL.md"
	LessonsFile = "LESSONS.md"
)

var idRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Info 列表项（供 MCP / FE / Eino Backend）。
type Info struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RelDir      string   `json:"relDir"`
	HasLessons  bool     `json:"hasLessons"`
	Enabled     bool     `json:"enabled"`                 // frontmatter enabled；缺省 true
	Globs       []string `json:"globs,omitempty"`         // 路径挂载（类 Cursor rules globs）
}

type frontMatter struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Globs       globsField `yaml:"globs"`
	Enabled     *bool      `yaml:"enabled,omitempty"`
}

// globsField 兼容 YAML 字符串或字符串列表。
type globsField []string

func (g *globsField) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	switch value.Kind {
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s != "" {
			*g = []string{s}
		}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		out := make([]string, 0, len(list))
		for _, s := range list {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		*g = out
		return nil
	default:
		return nil
	}
}

// Dir 返回项目 skills 根绝对路径。
func Dir(projectRoot string) string {
	return filepath.Join(filepath.Clean(projectRoot), RootDir)
}

// List 扫描 skills/*/SKILL.md（仅一层子目录）。
func List(projectRoot string) ([]Info, error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return nil, fmt.Errorf("empty project root")
	}
	base := Dir(root)
	ents, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]Info, 0, len(ents))
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		id := e.Name()
		relDir := filepath.ToSlash(filepath.Join(RootDir, id))
		skillPath := filepath.Join(base, id, SkillFile)
		raw, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}
		fm, _, err := parseSkillMD(string(raw))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(fm.Name)
		if name == "" {
			name = id
		}
		out = append(out, Info{
			ID:          id,
			Name:        name,
			Description: strings.TrimSpace(fm.Description),
			RelDir:      relDir,
			HasLessons:  false, // 由 App/MCP 以 lesson.HasAny 覆盖
			Enabled:     fmEnabled(fm.Enabled),
			Globs:       append([]string(nil), fm.Globs...),
		})
	}
	return out, nil
}

// ListEnabled 仅返回 frontmatter 启用的 Skill（Agent 热路径）。
func ListEnabled(projectRoot string) ([]Info, error) {
	list, err := List(projectRoot)
	if err != nil {
		return nil, err
	}
	return FilterEnabled(list), nil
}

// FilterEnabled 保留 Enabled=true 的项。
func FilterEnabled(list []Info) []Info {
	if len(list) == 0 {
		return list
	}
	out := make([]Info, 0, len(list))
	for _, s := range list {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}

func fmEnabled(p *bool) bool {
	if p == nil {
		return true
	}
	return *p
}

// LoadBody 读 SKILL.md 正文（经验在 lesson_entry，由 get_skill / InjectBrief 另附）。
func LoadBody(projectRoot, matched string) (info Info, body string, err error) {
	info, skillPath, err := resolve(projectRoot, matched)
	if err != nil {
		return Info{}, "", err
	}
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		return Info{}, "", err
	}
	_, content, err := parseSkillMD(string(raw))
	if err != nil {
		return Info{}, "", err
	}
	return info, strings.TrimSpace(content), nil
}

// Create 新建 skills/<id>/SKILL.md；已存在则报错。
func Create(projectRoot, id, name, description, content string) (Info, error) {
	rel, doc, err := RenderNew(id, name, description, content)
	if err != nil {
		return Info{}, err
	}
	id = strings.TrimSpace(id)
	skillPath := filepath.Join(projectRoot, filepath.FromSlash(rel))
	if _, err := os.Stat(skillPath); err == nil {
		return Info{}, fmt.Errorf("skill already exists: %s", id)
	}
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		return Info{}, err
	}
	if err := os.WriteFile(skillPath, []byte(doc), 0o644); err != nil {
		return Info{}, err
	}
	n := strings.TrimSpace(name)
	if n == "" {
		n = id
	}
	d := strings.TrimSpace(description)
	if d == "" {
		d = n
	}
	return Info{
		ID:          id,
		Name:        n,
		Description: d,
		RelDir:      filepath.ToSlash(filepath.Join(RootDir, id)),
		Enabled:     true,
	}, nil
}

// RenderSetEnabled 改 SKILL.md frontmatter 的 enabled（缺省/true 时省略键；false 写入）。
// 不落盘；App 经 WriteText 写回以走雷达。
func RenderSetEnabled(projectRoot, matched string, enabled bool) (relPath, doc string, info Info, err error) {
	info, skillPath, err := resolve(projectRoot, matched)
	if err != nil {
		return "", "", Info{}, err
	}
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		return "", "", Info{}, err
	}
	fmRaw, content, err := splitFrontmatter(string(raw))
	if err != nil {
		return "", "", Info{}, err
	}
	newFM, err := rewriteFrontmatterEnabled(fmRaw, enabled)
	if err != nil {
		return "", "", Info{}, err
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(newFM)
	if !strings.HasSuffix(newFM, "\n") {
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(content))
	b.WriteByte('\n')
	info.Enabled = enabled
	return RelSkillMD(info.ID), b.String(), info, nil
}

// RelSkillMD skills/<id>/SKILL.md 相对路径。
func RelSkillMD(id string) string {
	return filepath.ToSlash(filepath.Join(RootDir, id, SkillFile))
}

// Delete 删除 skills/<id>/ 整目录（不可恢复）。matched 为 id 或 name。
func Delete(projectRoot, matched string) (relDir string, err error) {
	info, _, err := resolve(projectRoot, matched)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(Dir(projectRoot), info.ID)
	if err := os.RemoveAll(abs); err != nil {
		return "", err
	}
	return info.RelDir, nil
}

// TreeRels 枚举 skills/<id>/ 下相对路径（含目录），删前登记 writetoken 用。
func TreeRels(projectRoot, matched string) (relDir string, rels []string, err error) {
	info, _, err := resolve(projectRoot, matched)
	if err != nil {
		return "", nil, err
	}
	abs := filepath.Join(Dir(projectRoot), info.ID)
	return info.RelDir, listRelTree(projectRoot, abs), nil
}

// listRelTree 枚举 abs 下相对 projectRoot 的路径（含目录）。
func listRelTree(projectRoot, abs string) []string {
	root := filepath.Clean(projectRoot)
	var out []string
	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "..") {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	return out
}

// RenderNew 生成新建 SKILL.md 文档（不落盘）。
func RenderNew(id, name, description, content string) (relPath string, doc string, err error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return "", "", fmt.Errorf("invalid skill id %q (use [a-zA-Z0-9_-], start alnum)", id)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	description = strings.TrimSpace(description)
	if description == "" {
		description = name
	}
	body := strings.TrimSpace(content)
	if body == "" {
		body = defaultNewSkillBody(name)
	}
	return RelSkillMD(id), formatSkillMD(name, description, body), nil
}

func defaultNewSkillBody(name string) string {
	return "# " + name + "\n\n" +
		"## 何时用\n\n" +
		"（补：触发语与任务类型）\n\n" +
		"## 本轮流程\n\n" +
		"1. `get_project` / `list_tree` — 现状\n" +
		"2. `read_file` — 按需读参考资料/邻文\n" +
		"3. `write_file` / `edit` — 见成功回执再说已落盘\n\n" +
		"## 启用\n\n" +
		"人 `pin_path` + 参考说明；你采信前馈「参考资料」。\n\n" +
		"## 不要\n\n" +
		"旁路写盘；替人打点；正文只留在本文件或 IDE 气泡。\n" +
		"作者规范：`get_skill skill-author`。\n"
}

// SkillAbsDir 返回 skill 目录绝对路径（Eino BaseDirectory）。
func SkillAbsDir(projectRoot, id string) string {
	return filepath.Join(Dir(projectRoot), id)
}

func resolve(projectRoot, matched string) (Info, string, error) {
	matched = strings.TrimSpace(matched)
	if matched == "" {
		return Info{}, "", fmt.Errorf("skill id/name required")
	}
	list, err := List(projectRoot)
	if err != nil {
		return Info{}, "", err
	}
	for _, info := range list {
		if info.ID == matched || info.Name == matched {
			return info, filepath.Join(Dir(projectRoot), info.ID, SkillFile), nil
		}
	}
	return Info{}, "", fmt.Errorf("skill not found: %s", matched)
}

func parseSkillMD(data string) (frontMatter, string, error) {
	fmRaw, content, err := splitFrontmatter(data)
	if err != nil {
		return frontMatter{}, "", err
	}
	var fm frontMatter
	if err := yaml.Unmarshal([]byte(fmRaw), &fm); err != nil {
		return frontMatter{}, "", fmt.Errorf("frontmatter: %w", err)
	}
	return fm, content, nil
}

// ParseSkillMD 解析 SKILL.md（供内置 packs 安装等产品层使用）。
func ParseSkillMD(data string) (name, description string, enabled bool, globs []string, body string, err error) {
	fm, body, err := parseSkillMD(data)
	if err != nil {
		return "", "", true, nil, "", err
	}
	return strings.TrimSpace(fm.Name), strings.TrimSpace(fm.Description), fmEnabled(fm.Enabled), append([]string(nil), fm.Globs...), body, nil
}

// ValidID 是否合法 Skill id。
func ValidID(id string) bool {
	return idRe.MatchString(strings.TrimSpace(id))
}

func splitFrontmatter(data string) (fm string, content string, err error) {
	const delim = "---"
	data = strings.TrimSpace(data)
	if !strings.HasPrefix(data, delim) {
		return "", "", fmt.Errorf("SKILL.md must start with YAML frontmatter (---)")
	}
	rest := data[len(delim):]
	endIdx := strings.Index(rest, "\n"+delim)
	if endIdx == -1 {
		return "", "", fmt.Errorf("frontmatter closing --- not found")
	}
	fm = strings.TrimSpace(rest[:endIdx])
	content = strings.TrimPrefix(rest[endIdx+len("\n"+delim):], "\n")
	return fm, content, nil
}

func formatSkillMD(name, description, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(name)
	b.WriteString("\n")
	b.WriteString("description: ")
	b.WriteString(yamlEscape(description))
	b.WriteString("\n---\n\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteByte('\n')
	return b.String()
}

func yamlEscape(s string) string {
	if strings.ContainsAny(s, ":#\n\"'") || strings.TrimSpace(s) != s {
		out, _ := yaml.Marshal(s)
		return strings.TrimSpace(string(out))
	}
	return s
}

func rewriteFrontmatterEnabled(fmRaw string, enabled bool) (string, error) {
	var m map[string]any
	if err := yaml.Unmarshal([]byte(fmRaw), &m); err != nil {
		return "", fmt.Errorf("frontmatter: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	if enabled {
		delete(m, "enabled")
	} else {
		m["enabled"] = false
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

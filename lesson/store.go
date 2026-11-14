// Package lesson 经验 SSOT：Markdown 文件（非 desk.db）。
// project → .ningharness/MEMORY.md；personal → ~/.ningharness/USER.md；skill → system/skills/<id>/LESSONS.md。
package lesson

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	StatusActive     = "active"
	StatusSuperseded = "superseded"
	StatusExpired    = "expired"

	ScopeSkill    = "skill"
	ScopeProject  = "project"
	ScopePersonal = "personal"

	InjectMaxRunes = 900

	// MemoryCharLimit / UserCharLimit 硬上限；超限 Append 报错（禁止静默丢条）。
	MemoryCharLimit = 2200
	UserCharLimit   = 1375

	skillsRoot      = "system/skills"
	lessonsFile     = "LESSONS.md"
	projectMemRel   = ".ningharness/MEMORY.md"
	personalDirName = ".ningharness"
	personalUserFile = "USER.md"
)

// Anchors 挂点（skill scope 必有 skillId）。
type Anchors struct {
	SkillID string `json:"skillId,omitempty"`
}

// Entry 一条经验（解析自 MD 节）。
type Entry struct {
	ID               string  `json:"id"`
	ProjectID        string  `json:"projectId,omitempty"`
	Scope            string  `json:"scope"`
	Anchors          Anchors `json:"anchors"`
	SkillID          string  `json:"skillId,omitempty"`
	Body             string  `json:"body"`
	Status           string  `json:"status"`
	SourceTaskID     string  `json:"sourceTaskId,omitempty"`
	ParentTaskID     string  `json:"parentTaskId,omitempty"`
	SourceSessionKey string  `json:"sourceSessionKey,omitempty"`
	SupersedesID     string  `json:"supersedesId,omitempty"`
	AckedAtMs        int64   `json:"ackedAtMs,omitempty"`
	CreatedAtMs      int64   `json:"createdAtMs"`
	UpdatedAtMs      int64   `json:"updatedAtMs,omitempty"`
}

// AppendInput 追加一条经验到对应 MD。
type AppendInput struct {
	Root             string
	Scope            string
	SkillID          string
	Body             string
	SourceTaskID     string
	ParentTaskID     string
	SourceSessionKey string
	SupersedesID     string
}

// Append 写入 MD SSOT；MEMORY/USER 超 cap 返回错误。
func Append(in AppendInput) (Entry, error) {
	body := strings.TrimSpace(in.Body)
	if body == "" {
		return Entry{}, fmt.Errorf("lesson: empty body")
	}
	scope := normalizeScope(in.Scope)
	if scope == "" {
		scope = ScopeSkill
	}
	skillID := strings.TrimSpace(in.SkillID)
	anchors := Anchors{}
	switch scope {
	case ScopeSkill:
		if skillID == "" {
			return Entry{}, fmt.Errorf("lesson: skill scope requires skill")
		}
		anchors.SkillID = skillID
	case ScopeProject, ScopePersonal:
	default:
		return Entry{}, fmt.Errorf("lesson: bad scope")
	}

	now := time.Now().UnixMilli()
	e := Entry{
		ID:               newID(),
		Scope:            scope,
		Anchors:          anchors,
		SkillID:          anchors.SkillID,
		Body:             body,
		Status:           StatusActive,
		SourceTaskID:     strings.TrimSpace(in.SourceTaskID),
		ParentTaskID:     strings.TrimSpace(in.ParentTaskID),
		SourceSessionKey: strings.TrimSpace(in.SourceSessionKey),
		SupersedesID:     strings.TrimSpace(in.SupersedesID),
		AckedAtMs:        now,
		CreatedAtMs:      now,
		UpdatedAtMs:      now,
	}

	path, err := filePath(in.Root, scope, skillID)
	if err != nil {
		return Entry{}, err
	}
	list, err := readFileEntries(path, scope, skillID)
	if err != nil {
		return Entry{}, err
	}
	if e.SupersedesID != "" {
		for i := range list {
			if list[i].ID == e.SupersedesID {
				list[i].Status = StatusSuperseded
				list[i].UpdatedAtMs = now
			}
		}
	}
	list = append(list, e)
	if err := writeFileEntries(path, scope, list); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// SetStatus 按 id 更新状态（在 root 项目与 personal 中查找）。
func SetStatus(root, id, status string) error {
	status = normalizeStatus(status)
	if status == "" {
		return fmt.Errorf("lesson: bad status")
	}
	return mutateEntry(root, id, func(e *Entry) {
		e.Status = status
		e.UpdatedAtMs = time.Now().UnixMilli()
	})
}

// Ack 标记认账时间（非进前馈门槛）。
func Ack(root, id string) error {
	return mutateEntry(root, id, func(e *Entry) {
		now := time.Now().UnixMilli()
		e.AckedAtMs = now
		e.UpdatedAtMs = now
	})
}

func ListActive(root string, skillIDs []string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 12
	}
	want := map[string]struct{}{}
	for _, id := range skillIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = struct{}{}
		}
	}
	var out []Entry
	if len(want) == 0 {
		// 无 skill 过滤时：扫所有 skill 包的 active
		base := filepath.Join(strings.TrimSpace(root), skillsRoot)
		ents, err := os.ReadDir(base)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, ent := range ents {
			if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
				continue
			}
			list, err := ListBySkill(root, ent.Name())
			if err != nil {
				continue
			}
			for _, e := range list {
				if e.Status != StatusActive {
					continue
				}
				out = append(out, e)
				if len(out) >= limit {
					return out, nil
				}
			}
		}
		return out, nil
	}
	for id := range want {
		list, err := ListBySkill(root, id)
		if err != nil {
			continue
		}
		for i := len(list) - 1; i >= 0; i-- {
			e := list[i]
			if e.Status != StatusActive {
				continue
			}
			out = append(out, e)
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

func ListBySkill(root, skillID string) ([]Entry, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return nil, fmt.Errorf("lesson: empty skill")
	}
	path, err := filePath(root, ScopeSkill, skillID)
	if err != nil {
		return nil, err
	}
	return readFileEntries(path, ScopeSkill, skillID)
}

func ListByScope(root, scope string, limit int) ([]Entry, error) {
	scope = normalizeScope(scope)
	if scope == "" {
		return nil, fmt.Errorf("lesson: bad scope")
	}
	if limit <= 0 {
		limit = 100
	}
	switch scope {
	case ScopeSkill:
		list, err := ListActive(root, nil, limit)
		return list, err
	case ScopeProject, ScopePersonal:
		path, err := filePath(root, scope, "")
		if err != nil {
			return nil, err
		}
		list, err := readFileEntries(path, scope, "")
		if err != nil {
			return nil, err
		}
		// 新→旧
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
		if len(list) > limit {
			list = list[:limit]
		}
		return list, nil
	default:
		return nil, fmt.Errorf("lesson: bad scope")
	}
}

func ListActivePersonal(limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 12
	}
	path, err := filePath("", ScopePersonal, "")
	if err != nil {
		return nil, err
	}
	list, err := readFileEntries(path, ScopePersonal, "")
	if err != nil {
		return nil, err
	}
	return filterActiveNewest(list, limit), nil
}

func ListActiveProject(root string, limit int) ([]Entry, error) {
	if limit <= 0 {
		limit = 12
	}
	path, err := filePath(root, ScopeProject, "")
	if err != nil {
		return nil, err
	}
	list, err := readFileEntries(path, ScopeProject, "")
	if err != nil {
		return nil, err
	}
	return filterActiveNewest(list, limit), nil
}

// HasActiveContaining 是否已有 active 正文包含 needle。
func HasActiveContaining(root, scope, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	scope = normalizeScope(scope)
	if scope == "" {
		scope = ScopeProject
	}
	var list []Entry
	var err error
	switch scope {
	case ScopeProject:
		list, err = ListActiveProject(root, 80)
	case ScopePersonal:
		list, err = ListActivePersonal(80)
	default:
		list, err = ListByScope(root, scope, 80)
	}
	if err != nil {
		return false
	}
	for _, e := range list {
		if e.Status == StatusActive && strings.Contains(e.Body, needle) {
			return true
		}
	}
	return false
}

func HasAny(root, skillID string) bool {
	list, err := ListBySkill(root, skillID)
	if err != nil {
		return false
	}
	for _, e := range list {
		if e.Status == StatusActive || e.Body != "" {
			return true
		}
	}
	return false
}

func CountProject(root string) int {
	n := 0
	if list, err := ListActiveProject(root, 1000); err == nil {
		n += len(list)
	}
	if list, err := ListActive(root, nil, 1000); err == nil {
		n += len(list)
	}
	return n
}

// InjectBrief 从 MD 注入有界前馈。
func InjectBrief(root string, skillIDs []string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = InjectMaxRunes
	}
	var b strings.Builder
	used := 0
	writeSection := func(title string, entries []Entry) {
		if len(entries) == 0 {
			return
		}
		var picked []Entry
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			if e.Status != "" && e.Status != StatusActive {
				continue
			}
			line := formatInjectLine(e)
			n := utf8.RuneCountInString(line) + 1
			if used+n > maxRunes && len(picked) > 0 {
				break
			}
			picked = append(picked, e)
			used += n
			if used >= maxRunes {
				break
			}
		}
		if len(picked) == 0 {
			return
		}
		for i, j := 0, len(picked)-1; i < j; i, j = i+1, j-1 {
			picked[i], picked[j] = picked[j], picked[i]
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(title)
		b.WriteByte('\n')
		for _, e := range picked {
			b.WriteString(formatInjectLine(e))
			b.WriteByte('\n')
		}
	}

	personal, _ := ListActivePersonal(8)
	writeSection("## 个人经验（USER.md · active）", personal)

	proj, _ := ListActiveProject(root, 8)
	writeSection("## 项目经验（MEMORY.md · active）", proj)

	if len(skillIDs) == 0 {
		n := CountProject(root)
		if b.Len() == 0 && n == 0 {
			return ""
		}
		if b.Len() == 0 {
			return fmt.Sprintf("## 项目经验\n有经验文件痕迹；本轮无路径匹配 skill，**不注入 skill 摘录**。需要时 list_lessons / get_skill。")
		}
		return strings.TrimSpace(b.String())
	}

	entries, err := ListActive(root, skillIDs, 40)
	if err != nil || len(entries) == 0 {
		if b.Len() == 0 {
			return "## 项目经验\n匹配 skill 无 active 条目；全文 list_lessons / get_skill。"
		}
		fmt.Fprintf(&b, "\n## Skill 经验\n匹配 skill 无 active 条目。\n")
		return strings.TrimSpace(b.String())
	}
	title := fmt.Sprintf("## Skill 经验（LESSONS.md · active）· 匹配 `%s`", strings.Join(skillIDs, "` · `"))
	writeSection(title, entries)
	return strings.TrimSpace(b.String())
}

// FormatForSkillBody 附在 get_skill 正文后供审。
func FormatForSkillBody(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## 技能经验（LESSONS.md · 全文供审；前馈只采信 active）\n\n")
	for _, e := range entries {
		ack := "未认账"
		if e.AckedAtMs > 0 {
			ack = "已认账"
		}
		fmt.Fprintf(&b, "### %s · %s · %s · %s\n\n", e.ID, e.Scope, e.Status, ack)
		if e.SourceTaskID != "" || e.ParentTaskID != "" {
			fmt.Fprintf(&b, "source_task: %s", e.SourceTaskID)
			if e.ParentTaskID != "" {
				fmt.Fprintf(&b, " · parent_task: %s", e.ParentTaskID)
			}
			b.WriteString("\n\n")
		}
		b.WriteString(e.Body)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// ExportMarkdown 导出 skill 作用域 LESSONS.md 正文。
func ExportMarkdown(entries []Entry) string {
	return renderMarkdown(ScopeSkill, entries)
}

// EnsureImported 兼容 no-op：MD 即 SSOT，无需从库导入。
func EnsureImported(root string) error {
	_ = root
	return nil
}

// ImportFromLessonsFile 将 MD 正文合并写入 skill LESSONS.md（保留已有 id）。
func ImportFromLessonsFile(root, skillID, md string) (int, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" {
		return 0, fmt.Errorf("lesson: empty skill")
	}
	incoming := parseMarkdownEntries(md, ScopeSkill, skillID)
	if len(incoming) == 0 {
		return 0, nil
	}
	path, err := filePath(root, ScopeSkill, skillID)
	if err != nil {
		return 0, err
	}
	existing, err := readFileEntries(path, ScopeSkill, skillID)
	if err != nil {
		return 0, err
	}
	have := map[string]struct{}{}
	for _, e := range existing {
		if e.ID != "" {
			have[e.ID] = struct{}{}
		}
	}
	n := 0
	now := time.Now().UnixMilli()
	for _, e := range incoming {
		if e.ID != "" {
			if _, ok := have[e.ID]; ok {
				continue
			}
		} else {
			e.ID = newID()
		}
		if e.CreatedAtMs == 0 {
			e.CreatedAtMs = now
		}
		if e.UpdatedAtMs == 0 {
			e.UpdatedAtMs = now
		}
		if e.AckedAtMs == 0 {
			e.AckedAtMs = now
		}
		e.Scope = ScopeSkill
		e.SkillID = skillID
		e.Anchors = Anchors{SkillID: skillID}
		existing = append(existing, e)
		have[e.ID] = struct{}{}
		n++
	}
	if n == 0 {
		return 0, nil
	}
	if err := writeFileEntries(path, ScopeSkill, existing); err != nil {
		return 0, err
	}
	return n, nil
}

func filePath(root, scope, skillID string) (string, error) {
	switch scope {
	case ScopeProject:
		root = strings.TrimSpace(root)
		if root == "" {
			return "", fmt.Errorf("lesson: empty root")
		}
		return filepath.Join(root, filepath.FromSlash(projectMemRel)), nil
	case ScopePersonal:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("lesson: home: %w", err)
		}
		return filepath.Join(home, personalDirName, personalUserFile), nil
	case ScopeSkill:
		root = strings.TrimSpace(root)
		if root == "" {
			return "", fmt.Errorf("lesson: empty root")
		}
		skillID = strings.TrimSpace(skillID)
		if skillID == "" {
			return "", fmt.Errorf("lesson: empty skill")
		}
		return filepath.Join(root, skillsRoot, skillID, lessonsFile), nil
	default:
		return "", fmt.Errorf("lesson: bad scope")
	}
}

func readFileEntries(path, scope, skillID string) ([]Entry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseMarkdownEntries(string(raw), scope, skillID), nil
}

func writeFileEntries(path, scope string, list []Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for i := range list {
		if list[i].ID == "" {
			list[i].ID = newID()
		}
		if list[i].CreatedAtMs == 0 {
			list[i].CreatedAtMs = now
		}
		if list[i].UpdatedAtMs == 0 {
			list[i].UpdatedAtMs = list[i].CreatedAtMs
		}
	}
	body := renderMarkdown(scope, list)
	if scope == ScopeProject || scope == ScopePersonal {
		capN := MemoryCharLimit
		if scope == ScopePersonal {
			capN = UserCharLimit
		}
		if utf8.RuneCountInString(body) > capN {
			return fmt.Errorf("lesson: %s exceeds %d chars (have %d); consolidate before append",
				filepath.Base(path), capN, utf8.RuneCountInString(body))
		}
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func renderMarkdown(scope string, entries []Entry) string {
	var b strings.Builder
	switch scope {
	case ScopeProject:
		b.WriteString("# MEMORY\n\n")
	case ScopePersonal:
		b.WriteString("# USER\n\n")
	default:
		b.WriteString("# LESSONS\n\n")
	}
	for _, e := range entries {
		st := e.Status
		if st == "" {
			st = StatusActive
		}
		ts := time.UnixMilli(e.CreatedAtMs).Format("2006-01-02 15:04")
		if e.CreatedAtMs == 0 {
			ts = time.Now().Format("2006-01-02 15:04")
		}
		fmt.Fprintf(&b, "## %s · %s\n\n", ts, st)
		fmt.Fprintf(&b, "<!-- lesson:%s -->\n", e.ID)
		if e.AckedAtMs > 0 {
			fmt.Fprintf(&b, "acked_at: %d\n", e.AckedAtMs)
		}
		if e.SourceTaskID != "" {
			fmt.Fprintf(&b, "source_task: %s\n", e.SourceTaskID)
		}
		if e.ParentTaskID != "" {
			fmt.Fprintf(&b, "parent_task: %s\n", e.ParentTaskID)
		}
		if e.SourceSessionKey != "" {
			fmt.Fprintf(&b, "source_session: %s\n", e.SourceSessionKey)
		}
		if e.SupersedesID != "" {
			fmt.Fprintf(&b, "supersedes: %s\n", e.SupersedesID)
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(e.Body))
		b.WriteString("\n\n")
	}
	return b.String()
}

func mutateEntry(root, id string, fn func(*Entry)) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("lesson: empty id")
	}
	type cand struct {
		path, scope, skillID string
	}
	var cands []cand
	if root = strings.TrimSpace(root); root != "" {
		cands = append(cands, cand{filepath.Join(root, filepath.FromSlash(projectMemRel)), ScopeProject, ""})
		base := filepath.Join(root, skillsRoot)
		if ents, err := os.ReadDir(base); err == nil {
			for _, ent := range ents {
				if !ent.IsDir() || strings.HasPrefix(ent.Name(), ".") {
					continue
				}
				cands = append(cands, cand{
					filepath.Join(base, ent.Name(), lessonsFile),
					ScopeSkill, ent.Name(),
				})
			}
		}
	}
	if p, err := filePath("", ScopePersonal, ""); err == nil {
		cands = append(cands, cand{p, ScopePersonal, ""})
	}
	for _, c := range cands {
		list, err := readFileEntries(c.path, c.scope, c.skillID)
		if err != nil {
			continue
		}
		found := false
		for i := range list {
			if list[i].ID == id {
				fn(&list[i])
				found = true
				break
			}
		}
		if !found {
			continue
		}
		return writeFileEntries(c.path, c.scope, list)
	}
	return fmt.Errorf("lesson: not found %s", id)
}

func filterActiveNewest(list []Entry, limit int) []Entry {
	var active []Entry
	for _, e := range list {
		if e.Status == "" || e.Status == StatusActive {
			active = append(active, e)
		}
	}
	if len(active) <= limit {
		// 返回最新在前
		for i, j := 0, len(active)-1; i < j; i, j = i+1, j-1 {
			active[i], active[j] = active[j], active[i]
		}
		return active
	}
	active = active[len(active)-limit:]
	for i, j := 0, len(active)-1; i < j; i, j = i+1, j-1 {
		active[i], active[j] = active[j], active[i]
	}
	return active
}

func newID() string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return "les_" + hex.EncodeToString(b[:])
}

func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case StatusActive, StatusSuperseded, StatusExpired:
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

func normalizeScope(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ScopeSkill, ScopeProject, ScopePersonal:
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return ""
	}
}

func formatInjectLine(e Entry) string {
	snip := trimRunes(e.Body, 220)
	ack := ""
	if e.AckedAtMs == 0 {
		ack = " · 未认账"
	}
	label := e.Scope
	if e.SkillID != "" {
		label = e.SkillID
	}
	return fmt.Sprintf("- `%s` · %s%s：%s", label, e.ID, ack, snip)
}

func trimRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

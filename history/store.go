// Package history 模型侧连贯上下文（history_message 表）。
package history

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"ningharness/store"
)

const (
	SnapshotStartMarker = "<!-- agentdesk-snapshot-start -->"
	SnapshotEndMarker   = "<!-- agentdesk-snapshot-end -->"
	historicalSnapNote  = "〔历史轮项目快照已省略；以最后一条 user 快照与磁盘为准〕"
)

// Msg 一行 history_message。
type Msg struct {
	Role          string // system|user|assistant|tool|thinking
	Content       string
	Feedforward   string // 前馈（仅 user 有意义：项目现状/引导等）；进模时拼进 WireUser，不进气泡
	ToolCallID      string
	ToolCallsJSON   string // assistant：[{"id","name","arguments","resource_ids"}]
	ResourceIDsJSON string // tool 行：关联 resource 表 id 列表
	TaskID          string
	Seq           int
	CreatedAtMs   int64
}

// ToolCallSpec 写入 assistant.tool_calls_json。
type ToolCallSpec struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Arguments   string  `json:"arguments"`
	ResourceIDs []int64 `json:"resource_ids,omitempty"`
}

// Budget 进模历史选项。
type Budget struct {
	// RecentKeep >0 时才截取最近 N 条；0=不截。
	RecentKeep int
	// MaxMsgRunes >0 时才截单条 content；0=不截。
	MaxMsgRunes int
}

// DefaultBudget 有界 ContextMessages：长会话不全量进模，避免每轮 Summarization 重压。
func DefaultBudget() Budget {
	return Budget{
		RecentKeep:  48,
		MaxMsgRunes: 8000,
	}
}

func (b Budget) normalize() Budget {
	return b
}

func openDB(root string) (*sql.DB, string, error) {
	db, err := store.OpenProject(root)
	if err != nil {
		return nil, "", err
	}
	return db, store.ProjectID(root), nil
}

// EnsureSystem 每个 session 头部保留/更新一条 system。
func EnsureSystem(root, sessionKey, systemText string) error {
	root = strings.TrimSpace(root)
	sessionKey = strings.TrimSpace(sessionKey)
	systemText = strings.TrimSpace(systemText)
	if root == "" || sessionKey == "" || systemText == "" {
		return fmt.Errorf("history: EnsureSystem requires root, sessionKey, system")
	}
	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	var id int64
	err = db.QueryRow(`SELECT id FROM history_message WHERE project_id=? AND session_key=? AND role='system' ORDER BY seq ASC LIMIT 1`,
		pid, sessionKey).Scan(&id)
	now := time.Now().UnixMilli()
	if err == sql.ErrNoRows {
		return insertMsg(db, pid, sessionKey, Msg{
			Role: "system", Content: systemText, CreatedAtMs: now,
		})
	}
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE history_message SET content=?, created_at_ms=? WHERE id=?`, systemText, now, id)
	return err
}

// Append 追加一行，自动分配 seq。
func Append(root, sessionKey string, m Msg) error {
	root = strings.TrimSpace(root)
	sessionKey = strings.TrimSpace(sessionKey)
	if root == "" || sessionKey == "" {
		return fmt.Errorf("history: Append requires root, sessionKey")
	}
	role := strings.TrimSpace(m.Role)
	if role == "" {
		return fmt.Errorf("history: role required")
	}
	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	if m.CreatedAtMs == 0 {
		m.CreatedAtMs = time.Now().UnixMilli()
	}
	m.Role = role
	return insertMsg(db, pid, sessionKey, m)
}

// AppendThinking 追加思考；同 task 连续 thinking 合并（对齐旧 StepBuilder）。
func AppendThinking(root, sessionKey, taskID, text string) error {
	root = strings.TrimSpace(root)
	sessionKey = strings.TrimSpace(sessionKey)
	taskID = strings.TrimSpace(taskID)
	if root == "" || sessionKey == "" {
		return fmt.Errorf("history: AppendThinking requires root, sessionKey")
	}
	if strings.TrimSpace(text) == "" {
		return nil
	}
	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	var id int64
	var role, existingTask string
	err = db.QueryRow(`SELECT id, role, task_id FROM history_message WHERE project_id=? AND session_key=? ORDER BY seq DESC, id DESC LIMIT 1`,
		pid, sessionKey).Scan(&id, &role, &existingTask)
	if err == nil && role == "thinking" && strings.TrimSpace(existingTask) == taskID {
		_, err = db.Exec(`UPDATE history_message SET content=content||?, created_at_ms=? WHERE id=?`,
			text, time.Now().UnixMilli(), id)
		return err
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return Append(root, sessionKey, Msg{
		Role: "thinking", Content: text, TaskID: taskID,
	})
}

func insertMsg(db *sql.DB, pid, sessionKey string, m Msg) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var maxSeq sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(seq) FROM history_message WHERE project_id=? AND session_key=?`, pid, sessionKey).Scan(&maxSeq); err != nil {
		return err
	}
	seq := 1
	if maxSeq.Valid {
		seq = int(maxSeq.Int64) + 1
	}
	ff := ""
	if strings.TrimSpace(m.Role) == "user" {
		ff = strings.TrimSpace(m.Feedforward)
	}
	_, err = tx.Exec(`INSERT INTO history_message(project_id, session_key, seq, role, content, feedforward, tool_call_id, tool_calls_json, resource_ids_json, task_id, created_at_ms)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		pid, sessionKey, seq, m.Role, m.Content, ff, strings.TrimSpace(m.ToolCallID), strings.TrimSpace(m.ToolCallsJSON),
		strings.TrimSpace(m.ResourceIDsJSON), strings.TrimSpace(m.TaskID), m.CreatedAtMs)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Load 全序加载。
func Load(root, sessionKey string) ([]Msg, error) {
	root = strings.TrimSpace(root)
	sessionKey = strings.TrimSpace(sessionKey)
	if root == "" || sessionKey == "" {
		return nil, nil
	}
	db, pid, err := openDB(root)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT seq, role, content, feedforward, tool_call_id, tool_calls_json, resource_ids_json, task_id, created_at_ms
		FROM history_message WHERE project_id=? AND session_key=? ORDER BY seq ASC, id ASC`, pid, sessionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMsgs(rows)
}

// LoadByTask 同一轮 Run 的行。
func LoadByTask(root, taskID string) ([]Msg, error) {
	root = strings.TrimSpace(root)
	taskID = strings.TrimSpace(taskID)
	if root == "" || taskID == "" {
		return nil, nil
	}
	db, pid, err := openDB(root)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT seq, role, content, feedforward, tool_call_id, tool_calls_json, resource_ids_json, task_id, created_at_ms
		FROM history_message WHERE project_id=? AND task_id=? ORDER BY seq ASC, id ASC`, pid, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMsgs(rows)
}

func scanMsgs(rows *sql.Rows) ([]Msg, error) {
	var out []Msg
	for rows.Next() {
		var m Msg
		if err := rows.Scan(&m.Seq, &m.Role, &m.Content, &m.Feedforward, &m.ToolCallID, &m.ToolCallsJSON, &m.ResourceIDsJSON, &m.TaskID, &m.CreatedAtMs); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ContentForUI 去掉项目快照外壳，只留用户可见正文（兼容旧 WrapUser 落盘）。
func ContentForUI(content string) string {
	content = strings.TrimSpace(content)
	start := strings.Index(content, SnapshotStartMarker)
	if start < 0 {
		return content
	}
	endRel := strings.Index(content[start:], SnapshotEndMarker)
	if endRel < 0 {
		return content
	}
	end := start + endRel + len(SnapshotEndMarker)
	before := strings.TrimSpace(content[:start])
	after := strings.TrimSpace(content[end:])
	if after != "" {
		return after
	}
	if before != "" {
		return before
	}
	return content
}

// ClearSession 清空某会话全部 history（含 system/tool）。
func ClearSession(root, sessionKey string) error {
	root = strings.TrimSpace(root)
	sessionKey = strings.TrimSpace(sessionKey)
	if root == "" || sessionKey == "" {
		return fmt.Errorf("history: ClearSession requires root, sessionKey")
	}
	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM history_message WHERE project_id=? AND session_key=?`, pid, sessionKey)
	return err
}

// TruncateAfterUIKeep 按 UI 可见消息条数截断（保留前 keep 条 user/有正文 assistant，及其之前的 tool/system）。
func TruncateAfterUIKeep(root, sessionKey string, keep int) error {
	root = strings.TrimSpace(root)
	sessionKey = strings.TrimSpace(sessionKey)
	if root == "" || sessionKey == "" {
		return fmt.Errorf("history: TruncateAfterUIKeep requires root, sessionKey")
	}
	if keep < 0 {
		keep = 0
	}
	msgs, err := Load(root, sessionKey)
	if err != nil {
		return err
	}
	if keep == 0 {
		return ClearSession(root, sessionKey)
	}
	ui := 0
	cutoff := 0
	found := false
	for _, m := range msgs {
		if !IsUIRole(m) {
			continue
		}
		ui++
		if ui == keep {
			cutoff = m.Seq
			found = true
			break
		}
	}
	if !found {
		return nil // keep >= UI 条数，无需截
	}
	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM history_message WHERE project_id=? AND session_key=? AND seq>?`, pid, sessionKey, cutoff)
	return err
}

// IsUIRole 是否进入会话 UI（跳过 system/tool/thinking 与仅 tool_calls 的空 assistant）。
func IsUIRole(m Msg) bool {
	switch strings.TrimSpace(m.Role) {
	case "user":
		return strings.TrimSpace(m.Content) != ""
	case "assistant":
		return strings.TrimSpace(m.Content) != ""
	default:
		return false
	}
}

// ApplyFeedforward 前馈 + 用户原话 → 进模 user content（markers 仅作 wire 分隔，新落盘走 feedforward 列）。
func ApplyFeedforward(feedforward, userPrompt string) string {
	userPrompt = strings.TrimSpace(userPrompt)
	feedforward = strings.TrimSpace(feedforward)
	if feedforward == "" {
		return userPrompt
	}
	var b strings.Builder
	b.WriteString(SnapshotStartMarker)
	b.WriteByte('\n')
	b.WriteString(feedforward)
	b.WriteByte('\n')
	b.WriteString(SnapshotEndMarker)
	if userPrompt != "" {
		b.WriteString("\n\n")
		b.WriteString(userPrompt)
	}
	return b.String()
}

// WrapUser 兼容旧名；等价 ApplyFeedforward。
func WrapUser(feedforward, userPrompt string) string {
	return ApplyFeedforward(feedforward, userPrompt)
}

// WireUser 单条进模 user 正文：优先 feedforward 列，否则兼容 content 内嵌旧快照。
func WireUser(m Msg) string {
	if ff := strings.TrimSpace(m.Feedforward); ff != "" {
		return ApplyFeedforward(ff, m.Content)
	}
	return strings.TrimSpace(m.Content)
}

// EncodeToolCalls 序列化 tool_calls_json。
func EncodeToolCalls(calls []ToolCallSpec) string {
	if len(calls) == 0 {
		return ""
	}
	raw, err := json.Marshal(calls)
	if err != nil {
		return ""
	}
	return string(raw)
}

// BuildForModel 进模前处理：ContextMessages 不带旧轮前馈；瘦旧 content 内嵌快照；跳过 thinking；修 tool 链。
func BuildForModel(msgs []Msg, budget Budget, skipLeadingSystem bool) []Msg {
	_ = budget.normalize()
	if len(msgs) == 0 {
		return nil
	}
	// ContextMessages：只用 content（剥旧嵌套快照），忽略 feedforward 列——前馈只服务触发当轮。
	normalized := make([]Msg, 0, len(msgs))
	for _, m := range msgs {
		cp := m
		if strings.TrimSpace(cp.Role) == "user" {
			cp.Content = ContentForUI(cp.Content)
			cp.Feedforward = ""
		}
		if strings.TrimSpace(cp.Role) == "tool" {
			cp.Content = WireTool(cp)
		}
		normalized = append(normalized, cp)
	}
	out := StripHistoricalSnapshots(normalized)
	filtered := make([]Msg, 0, len(out))
	for _, m := range out {
		if strings.TrimSpace(m.Role) == "thinking" {
			continue
		}
		filtered = append(filtered, m)
	}
	out = filtered
	if skipLeadingSystem {
		for len(out) > 0 && strings.TrimSpace(out[0].Role) == "system" {
			out = out[1:]
		}
	}
	if budget.RecentKeep > 0 && len(out) > budget.RecentKeep {
		start := len(out) - budget.RecentKeep
		// 勿从 tool 行切开
		for start > 0 && out[start].Role == "tool" {
			start--
		}
		out = out[start:]
	}
	out = RepairChain(out)
	if budget.MaxMsgRunes > 0 {
		trimmed := make([]Msg, len(out))
		copy(trimmed, out)
		for i := range trimmed {
			trimmed[i].Content = trimRunes(trimmed[i].Content, budget.MaxMsgRunes)
		}
		return trimmed
	}
	return out
}

// RepairChain 保证 assistant.tool_calls 后有对应 tool；残缺则去掉 tool_calls 或丢孤儿 tool。
func RepairChain(msgs []Msg) []Msg {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]Msg, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "tool" {
			if !validToolAfter(out, m) {
				continue
			}
		}
		out = append(out, m)
	}
	return stripUnfulfilledToolCalls(out)
}

func validToolAfter(prefix []Msg, tool Msg) bool {
	if len(prefix) == 0 {
		return false
	}
	prev := prefix[len(prefix)-1]
	id := strings.TrimSpace(tool.ToolCallID)
	if prev.Role == "assistant" && strings.TrimSpace(prev.ToolCallsJSON) != "" {
		return toolCallIDInJSON(prev.ToolCallsJSON, id)
	}
	if prev.Role == "tool" {
		asst := assistantForToolGroup(prefix)
		return asst != nil && toolCallIDInJSON(asst.ToolCallsJSON, id)
	}
	return false
}

func assistantForToolGroup(msgs []Msg) *Msg {
	for i := len(msgs) - 1; i >= 0; i-- {
		switch msgs[i].Role {
		case "tool":
			continue
		case "assistant":
			if strings.TrimSpace(msgs[i].ToolCallsJSON) != "" {
				cp := msgs[i]
				return &cp
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func toolCallIDInJSON(raw, id string) bool {
	id = strings.TrimSpace(id)
	calls := decodeToolCallIDs(raw)
	if id == "" {
		return len(calls) > 0
	}
	for _, c := range calls {
		if c == id {
			return true
		}
	}
	return false
}

func decodeToolCallIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var specs []ToolCallSpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		return nil
	}
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		if id := strings.TrimSpace(s.ID); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func stripUnfulfilledToolCalls(msgs []Msg) []Msg {
	out := make([]Msg, 0, len(msgs))
	for i := 0; i < len(msgs); i++ {
		m := msgs[i]
		if m.Role == "assistant" && strings.TrimSpace(m.ToolCallsJSON) != "" {
			needed := decodeToolCallIDs(m.ToolCallsJSON)
			if !toolResultsFollow(msgs, i+1, needed) {
				m.ToolCallsJSON = ""
				if strings.TrimSpace(m.Content) == "" {
					// 空 assistant+残缺 tool_calls：丢掉，避免污染
					continue
				}
			}
		}
		out = append(out, m)
	}
	return out
}

func toolResultsFollow(msgs []Msg, start int, needed []string) bool {
	if len(needed) == 0 || start >= len(msgs) {
		return false
	}
	pending := make(map[string]struct{}, len(needed))
	for _, id := range needed {
		pending[id] = struct{}{}
	}
	for j := start; j < len(msgs); j++ {
		if msgs[j].Role != "tool" {
			break
		}
		id := strings.TrimSpace(msgs[j].ToolCallID)
		delete(pending, id)
	}
	return len(pending) == 0
}

// StripHistoricalSnapshots 仅保留最后一条含快照的 user 的完整快照。
func StripHistoricalSnapshots(msgs []Msg) []Msg {
	if len(msgs) == 0 {
		return msgs
	}
	keepIdx := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" && hasSnapshot(msgs[i].Content) {
			keepIdx = i
			break
		}
	}
	if keepIdx < 0 {
		return msgs
	}
	out := make([]Msg, len(msgs))
	copy(out, msgs)
	changed := false
	for i := range out {
		if out[i].Role != "user" || i == keepIdx {
			continue
		}
		if stripped, ok := stripSnapshot(out[i].Content); ok {
			out[i].Content = stripped
			changed = true
		}
	}
	if !changed {
		return msgs
	}
	return out
}

func hasSnapshot(content string) bool {
	return strings.Contains(content, SnapshotStartMarker)
}

func stripSnapshot(content string) (string, bool) {
	start := strings.Index(content, SnapshotStartMarker)
	if start < 0 {
		return content, false
	}
	endRel := strings.Index(content[start:], SnapshotEndMarker)
	if endRel < 0 {
		return content, false
	}
	end := start + endRel + len(SnapshotEndMarker)
	before := strings.TrimSpace(content[:start])
	after := strings.TrimSpace(content[end:])
	var b strings.Builder
	if before != "" {
		b.WriteString(before)
		b.WriteByte('\n')
	}
	b.WriteString(historicalSnapNote)
	if after != "" {
		b.WriteByte('\n')
		b.WriteString(after)
	}
	return strings.TrimSpace(b.String()), true
}

func trimRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

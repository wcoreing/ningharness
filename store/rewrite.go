package store

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// 逻辑表名（长名优先替换，避免 history_message 误伤 history_message_fts）。
var logicalTables = []string{
	"history_message_fts",
	"history_message",
	"job_steps",
	"lesson_entry",
	"schema_version",
	"timeline_reviews",
	"timeline_rounds",
	"file_timeline",
	"review_sessions",
	"pin_sessions",
	"growth_reflect",
	"library_pack",
	"tool_payload",
	"queue_steps",
	"queue_tasks",
	"run_events",
	"run_tools",
	"projects",
	"sessions",
	"resource",
	"tasks",
	"jobs",
	"meta",
	"pins",
	"runs",
}

var (
	rewriteMu     sync.Mutex
	rewritePrefix string
	rewriteRe     []*regexp.Regexp
	rewriteTo     []string

	reOnConflict = regexp.MustCompile(`(?is)\s*ON CONFLICT\([^)]*\)\s*DO UPDATE SET\s*`)
	reExcluded   = regexp.MustCompile(`(?i)excluded\.([a-zA-Z0-9_]+)`)
	reJSONExt    = regexp.MustCompile(`(?i)json_extract\(\s*([a-zA-Z0-9_.]+)\s*,\s*'([^']+)'\s*\)`)
	reConcat     = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)\|\|(\?)`)
)

func buildRewriters(prefix string) {
	sorted := append([]string(nil), logicalTables...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i]) > len(sorted[j]) })
	rewriteRe = make([]*regexp.Regexp, 0, len(sorted))
	rewriteTo = make([]string, 0, len(sorted))
	for _, n := range sorted {
		rewriteRe = append(rewriteRe, regexp.MustCompile(`\b`+regexp.QuoteMeta(n)+`\b`))
		rewriteTo = append(rewriteTo, prefix+n)
	}
}

// RewriteSQL 将逻辑表名加上 TablePrefix。无前缀时原样返回。
func RewriteSQL(q string) string {
	p := TablePrefix()
	if p == "" || q == "" {
		return q
	}
	rewriteMu.Lock()
	defer rewriteMu.Unlock()
	if p != rewritePrefix {
		buildRewriters(p)
		rewritePrefix = p
	}
	out := q
	for i, re := range rewriteRe {
		out = re.ReplaceAllString(out, rewriteTo[i])
	}
	return out
}

// AdaptSQL 表前缀 + MySQL 方言改写（ON CONFLICT / INSERT OR / json_extract / ||）。
func AdaptSQL(q string) string {
	q = RewriteSQL(q)
	if !IsMySQL() || q == "" {
		return q
	}
	q = strings.ReplaceAll(q, "INSERT OR IGNORE", "INSERT IGNORE")
	if loc := reOnConflict.FindStringIndex(q); loc != nil {
		head := q[:loc[0]]
		tail := q[loc[1]:]
		tail = reExcluded.ReplaceAllString(tail, "VALUES($1)")
		q = head + " ON DUPLICATE KEY UPDATE " + tail
	}
	q = reJSONExt.ReplaceAllString(q, "JSON_UNQUOTE(JSON_EXTRACT($1, '$2'))")
	q = reConcat.ReplaceAllString(q, "CONCAT($1, $2)")
	// meta.key 为 MySQL 保留字
	meta := T("meta")
	q = strings.ReplaceAll(q, meta+"(project_id, key, value)", meta+"(project_id, `key`, value)")
	q = strings.ReplaceAll(q, meta+"(project_id, key, value)", meta+"(project_id, `key`, value)")
	q = strings.ReplaceAll(q, " "+meta+"(project_id, key,", " "+meta+"(project_id, `key`,")
	return q
}

// R 短名：AdaptSQL（域包 SQL 入口）。
func R(q string) string { return AdaptSQL(q) }

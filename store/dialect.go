package store

import (
	"fmt"
	"strings"
)

// Upsert 生成 INSERT ... ON CONFLICT / ON DUPLICATE KEY UPDATE。
// cols 含全部写入列；conflict 为冲突键；updates 为冲突时更新列（须是 cols 子集）。
func Upsert(table string, cols, conflict, updates []string) string {
	t := T(table)
	ph := placeholders(len(cols))
	colList := strings.Join(cols, ", ")
	if IsMySQL() {
		var sets []string
		for _, u := range updates {
			sets = append(sets, fmt.Sprintf("%s=VALUES(%s)", u, u))
		}
		return fmt.Sprintf(
			"INSERT INTO %s(%s) VALUES(%s) ON DUPLICATE KEY UPDATE %s",
			t, colList, ph, strings.Join(sets, ", "),
		)
	}
	var sets []string
	for _, u := range updates {
		sets = append(sets, fmt.Sprintf("%s=excluded.%s", u, u))
	}
	return fmt.Sprintf(
		"INSERT INTO %s(%s) VALUES(%s) ON CONFLICT(%s) DO UPDATE SET %s",
		t, colList, ph, strings.Join(conflict, ", "), strings.Join(sets, ", "),
	)
}

// InsertIgnore 生成 INSERT OR IGNORE / INSERT IGNORE。
func InsertIgnore(table string, cols []string) string {
	t := T(table)
	ph := placeholders(len(cols))
	colList := strings.Join(cols, ", ")
	if IsMySQL() {
		return fmt.Sprintf("INSERT IGNORE INTO %s(%s) VALUES(%s)", t, colList, ph)
	}
	return fmt.Sprintf("INSERT OR IGNORE INTO %s(%s) VALUES(%s)", t, colList, ph)
}

// ConcatExpr 列与占位符拼接：sqlite content||? ；mysql CONCAT(content, ?)。
func ConcatExpr(column, placeholder string) string {
	if IsMySQL() {
		return fmt.Sprintf("CONCAT(%s, %s)", column, placeholder)
	}
	return column + "||" + placeholder
}

// JSONExtract 取 JSON 路径表达式（用于 WHERE/SELECT）。
func JSONExtract(column, path string) string {
	if IsMySQL() {
		return fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(%s, '%s'))", column, path)
	}
	return fmt.Sprintf("json_extract(%s, '%s')", column, path)
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

package store

import "testing"

func TestAdaptSQL_MySQL(t *testing.T) {
	Configure(Config{Driver: DriverMySQL, TablePrefix: "nh_"})
	t.Cleanup(func() { Configure(Config{}) })

	in := `INSERT INTO sessions(project_id, id, title, orch_key, updated_at_ms) VALUES(?,?,?,?,?)
		ON CONFLICT(project_id, id) DO UPDATE SET title=excluded.title, updated_at_ms=excluded.updated_at_ms`
	out := AdaptSQL(in)
	if !containsAll(out, "nh_sessions", "ON DUPLICATE KEY UPDATE", "VALUES(title)", "VALUES(updated_at_ms)") {
		t.Fatalf("unexpected: %s", out)
	}

	in2 := `UPDATE history_message SET content=content||?, created_at_ms=? WHERE id=?`
	out2 := AdaptSQL(in2)
	if !containsAll(out2, "nh_history_message", "CONCAT(content, ?)") {
		t.Fatalf("unexpected concat: %s", out2)
	}

	in3 := `SELECT * FROM lesson_entry WHERE json_extract(anchors,'$.skillId')=?`
	out3 := AdaptSQL(in3)
	if !containsAll(out3, "nh_lesson_entry", "JSON_UNQUOTE(JSON_EXTRACT(anchors, '$.skillId'))") {
		t.Fatalf("unexpected json: %s", out3)
	}
}

func TestAdaptSQL_SQLiteNoopPrefix(t *testing.T) {
	Configure(Config{Driver: DriverSQLite})
	t.Cleanup(func() { Configure(Config{}) })
	in := `SELECT * FROM sessions WHERE project_id=?`
	if AdaptSQL(in) != in {
		t.Fatalf("sqlite should keep SQL: %s", AdaptSQL(in))
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !stringContains(s, p) {
			return false
		}
	}
	return true
}

func stringContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

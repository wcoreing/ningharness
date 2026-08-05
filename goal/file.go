package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func Serialize(f File) string {
	b, err := yaml.Marshal(File{
		Objective: f.Objective,
		Status:    f.Status,
	})
	if err != nil {
		return fmt.Sprintf("objective: %s\nstatus: %s\n", f.Objective, f.Status)
	}
	return string(b)
}

func WriteOnce(path, objective string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("goal: empty control path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := Serialize(File{Objective: strings.TrimSpace(objective), Status: StatusActive})
	return os.WriteFile(path, []byte(body), 0o644)
}

func ReadStatus(path string) Status {
	raw, err := os.ReadFile(path)
	if err != nil {
		return StatusBlocked
	}
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return StatusBlocked
	}
	switch Status(strings.TrimSpace(string(f.Status))) {
	case StatusActive, StatusComplete:
		return Status(strings.TrimSpace(string(f.Status)))
	default:
		return StatusBlocked
	}
}

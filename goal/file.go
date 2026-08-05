package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func Serialize(f File) string {
	out := File{
		Objective: f.Objective,
		Status:    f.Status,
		Next:      strings.TrimSpace(f.Next),
	}
	b, err := yaml.Marshal(out)
	if err != nil {
		s := fmt.Sprintf("objective: %s\nstatus: %s\n", f.Objective, f.Status)
		if out.Next != "" {
			s += "next: " + out.Next + "\n"
		}
		return s
	}
	return string(b)
}

func WriteOnce(path, objective string) error {
	return WriteFile(path, File{Objective: strings.TrimSpace(objective), Status: StatusActive})
}

// WriteFile 写入 GOAL.yaml 控制面。
func WriteFile(path string, f File) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("goal: empty control path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f.Objective = strings.TrimSpace(f.Objective)
	f.Next = strings.TrimSpace(f.Next)
	if f.Status == "" {
		f.Status = StatusActive
	}
	return os.WriteFile(path, []byte(Serialize(f)), 0o644)
}

// ReadFile 读控制面；失败返回零值与 error。
func ReadFile(path string) (File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return File{}, err
	}
	f.Objective = strings.TrimSpace(f.Objective)
	f.Next = strings.TrimSpace(f.Next)
	f.Status = Status(strings.TrimSpace(string(f.Status)))
	return f, nil
}

func ReadStatus(path string) Status {
	f, err := ReadFile(path)
	if err != nil {
		return StatusBlocked
	}
	switch f.Status {
	case StatusActive, StatusComplete:
		return f.Status
	default:
		return StatusBlocked
	}
}

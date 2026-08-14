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
		Reason:    strings.TrimSpace(f.Reason),
		Next:      strings.TrimSpace(f.Next),
	}
	b, err := yaml.Marshal(out)
	if err != nil {
		s := fmt.Sprintf("objective: %s\nstatus: %s\n", f.Objective, f.Status)
		if out.Reason != "" {
			s += "reason: " + out.Reason + "\n"
		}
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
	f.Reason = strings.TrimSpace(f.Reason)
	f.Next = strings.TrimSpace(f.Next)
	if f.Status == "" {
		f.Status = StatusActive
	}
	if f.Status != StatusBlocked {
		f.Reason = ""
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
	f.Reason = strings.TrimSpace(f.Reason)
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

// ReadReason 读 blocked 原因；非 blocked 或读失败返回空。
func ReadReason(path string) string {
	f, err := ReadFile(path)
	if err != nil || f.Status != StatusBlocked {
		return ""
	}
	return strings.TrimSpace(f.Reason)
}

// Resume 将 blocked 控制面改回 active并清空 reason，供人工续跑。
func Resume(path string) error {
	f, err := ReadFile(path)
	if err != nil {
		return err
	}
	if f.Status != StatusBlocked {
		return fmt.Errorf("goal: status %q is not blocked", f.Status)
	}
	f.Status = StatusActive
	f.Reason = ""
	return WriteFile(path, f)
}

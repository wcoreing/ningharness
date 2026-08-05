package job

import (
	"fmt"
	"strings"
)

func FormatSteerBlock(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return fmt.Sprintf("[user_steering]\n%s\n[/user_steering]", text)
}

func FormatSteerForSession(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return "【运行中插话】\n" + text
}

func appendSteerPending(prev, add string) string {
	prev = strings.TrimSpace(prev)
	add = strings.TrimSpace(add)
	if add == "" {
		return prev
	}
	if prev == "" {
		return add
	}
	return prev + "\n\n" + add
}

func (m *Manager) Steer(jobID, text string) (Job, error) {
	if m == nil {
		return Job{}, fmt.Errorf("job: nil manager")
	}
	jobID = strings.TrimSpace(jobID)
	text = strings.TrimSpace(text)
	if text == "" {
		return Job{}, fmt.Errorf("job: empty steer")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(m.root) == "" {
		return Job{}, fmt.Errorf("job: no project")
	}
	idx := -1
	if jobID != "" {
		for i := range m.file.Jobs {
			if m.file.Jobs[i].ID == jobID {
				idx = i
				break
			}
		}
		if idx < 0 {
			return Job{}, fmt.Errorf("job: not found %s", jobID)
		}
	} else {
		for i := range m.file.Jobs {
			if m.file.Jobs[i].Status == StatusRunning {
				idx = i
				break
			}
		}
		if idx < 0 {
			return Job{}, fmt.Errorf("job: no running task to steer")
		}
	}
	t := &m.file.Jobs[idx]
	switch t.Status {
	case StatusRunning, StatusQueued:
	default:
		return Job{}, fmt.Errorf("job: cannot steer status=%s", t.Status)
	}
	t.SteerPending = appendSteerPending(t.SteerPending, text)
	if t.Status == StatusRunning {
		hint := strings.TrimSpace(t.ProgressHint)
		if hint == "" {
			t.ProgressHint = "已排队插话"
		} else if !strings.Contains(hint, "插话") {
			t.ProgressHint = hint + " · 插话待送达"
		}
	}
	out := cloneJob(*t)
	_ = saveFile(m.root, m.file)
	m.emitLocked()
	return out, nil
}

func (m *Manager) TakeSteerPending(jobID string) string {
	if m == nil {
		return ""
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != jobID {
			continue
		}
		text := strings.TrimSpace(m.file.Jobs[i].SteerPending)
		if text == "" {
			return ""
		}
		m.file.Jobs[i].SteerPending = ""
		hint := m.file.Jobs[i].ProgressHint
		hint = strings.ReplaceAll(hint, " · 插话待送达", "")
		hint = strings.TrimSpace(strings.ReplaceAll(hint, "已排队插话", ""))
		m.file.Jobs[i].ProgressHint = hint
		_ = saveFile(m.root, m.file)
		m.emitLocked()
		return text
	}
	return ""
}

func (m *Manager) takeSteerPendingLocked(jobID string) string {
	for i := range m.file.Jobs {
		if m.file.Jobs[i].ID != jobID {
			continue
		}
		text := strings.TrimSpace(m.file.Jobs[i].SteerPending)
		if text == "" {
			return ""
		}
		m.file.Jobs[i].SteerPending = ""
		hint := m.file.Jobs[i].ProgressHint
		hint = strings.ReplaceAll(hint, " · 插话待送达", "")
		hint = strings.TrimSpace(strings.ReplaceAll(hint, "已排队插话", ""))
		m.file.Jobs[i].ProgressHint = hint
		return text
	}
	return ""
}

func (m *Manager) flushOrphanSteerLocked(t *Job) {
	if t == nil {
		return
	}
	if strings.TrimSpace(t.SteerPending) == "" {
		return
	}
	t.SteerPending = ""
}

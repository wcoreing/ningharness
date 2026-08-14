package job

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func cloneJob(j Job) Job {
	if len(j.Steps) == 0 {
		return j
	}
	steps := make([]Step, len(j.Steps))
	copy(steps, j.Steps)
	j.Steps = steps
	return j
}

func clampParallel(n int) int {
	if n < 1 {
		return DefaultMaxParallel
	}
	if n > 8 {
		return 8
	}
	return n
}

func trimReason(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if max <= 0 || len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func newID(prefix string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixMilli(), hex.EncodeToString(b[:]))
}

func titleFromPrompt(prompt, targetRel string) string {
	if targetRel != "" {
		return filepath.Base(targetRel)
	}
	r := []rune(strings.TrimSpace(prompt))
	if len(r) > 36 {
		return string(r[:36]) + "…"
	}
	if len(r) == 0 {
		return "agent-turn"
	}
	return string(r)
}

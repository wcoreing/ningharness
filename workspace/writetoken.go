// 写盘 token：关联本端写盘与 fsnotify。
package workspace

import (
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type pathKey struct {
	projectID string
	path      string
}

type tokenEntry struct {
	writeID   string
	expiresAt time.Time
}

var (
	mu        sync.Mutex
	pathIndex = map[pathKey]tokenEntry{}
)

const entryTTL = 30 * time.Second

func normalizePath(p string) string {
	return filepath.ToSlash(strings.TrimSpace(p))
}

// Register 写盘前/后登记路径 → writeId。
func Register(projectID, writeID string, paths []string) {
	pid := strings.TrimSpace(projectID)
	wid := strings.TrimSpace(writeID)
	if pid == "" || wid == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	purgeExpiredLocked(time.Now())
	exp := time.Now().Add(entryTTL)
	for _, p := range paths {
		p = normalizePath(p)
		if p == "" {
			continue
		}
		pathIndex[pathKey{pid, p}] = tokenEntry{writeID: wid, expiresAt: exp}
	}
}

// Resolve 解析本次变更是否来自本端写盘；命中则消费并返回 writeId。
func Resolve(projectID string, relPaths []string) string {
	pid := strings.TrimSpace(projectID)
	if pid == "" || len(relPaths) == 0 {
		return ""
	}
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	purgeExpiredLocked(now)

	var writeID string
	matched := make([]string, 0, len(relPaths))
	for _, p := range relPaths {
		p = normalizePath(p)
		if p == "" {
			continue
		}
		e, ok := pathIndex[pathKey{pid, p}]
		if !ok || e.writeID == "" || now.After(e.expiresAt) {
			continue
		}
		if writeID == "" {
			writeID = e.writeID
		} else if e.writeID != writeID {
			// 混合来源：当作外部变更
			return ""
		}
		matched = append(matched, p)
	}
	for _, p := range matched {
		delete(pathIndex, pathKey{pid, p})
	}
	return writeID
}

func purgeExpiredLocked(now time.Time) {
	for k, e := range pathIndex {
		if now.After(e.expiresAt) {
			delete(pathIndex, k)
		}
	}
}

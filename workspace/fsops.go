package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ningharness/writetoken"
)

// MutationResult 单条/批量路径操作结果。
type MutationResult struct {
	OK      []string          `json:"ok"`
	Failed  map[string]string `json:"failed,omitempty"`
	MovedTo map[string]string `json:"movedTo,omitempty"` // from → to（Move/BatchMove/Rename）
}

func (s *Service) registerWrite(p *Project, writeID string, rels []string) {
	if p == nil || strings.TrimSpace(writeID) == "" || len(rels) == 0 {
		return
	}
	clean := make([]string, 0, len(rels))
	for _, r := range rels {
		r = filepath.ToSlash(strings.TrimSpace(r))
		if r != "" {
			clean = append(clean, r)
		}
	}
	if len(clean) > 0 {
		writetoken.Register(p.ID, writeID, clean)
	}
}

func (s *Service) currentProject() (*Project, error) {
	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	if p == nil {
		return nil, fmt.Errorf("no project open")
	}
	return p, nil
}

func normalizeRel(relPath string) (string, error) {
	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(relPath))))
	if rel == "." || rel == "" || rel == "/" {
		return "", fmt.Errorf("empty relPath")
	}
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return "", fmt.Errorf("path escapes project: %s", relPath)
	}
	return strings.TrimPrefix(rel, "/"), nil
}

func isPathInside(parent, child string) bool {
	p := filepath.ToSlash(parent)
	c := filepath.ToSlash(child)
	if p == c {
		return true
	}
	return strings.HasPrefix(c, p+"/")
}

func uniqueDestName(parentAbs, base string) string {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 0; i <= 9999; i++ {
		var name string
		switch {
		case i == 0:
			name = base
		case i == 1:
			name = stem + " 副本" + ext
		default:
			name = fmt.Sprintf("%s 副本-%d%s", stem, i, ext)
		}
		if _, err := os.Stat(filepath.Join(parentAbs, name)); os.IsNotExist(err) {
			return name
		}
	}
	return fmt.Sprintf("%s 副本-%d%s", stem, time.Now().UnixNano(), ext)
}

// Mkdir 创建目录；已存在则报错。
func (s *Service) Mkdir(relPath, writeID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.mkdirLocked(relPath, writeID)
}

func (s *Service) mkdirLocked(relPath, writeID string) error {
	p, err := s.currentProject()
	if err != nil {
		return err
	}
	rel, err := normalizeRel(relPath)
	if err != nil {
		return err
	}
	abs, err := resolveIn(p, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("已存在: %s", rel)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}
	s.registerWrite(p, writeID, []string{rel})
	return nil
}

// CreateFile 创建空文件；父目录自动创建；已存在则报错。
func (s *Service) CreateFile(relPath, writeID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.createFileLocked(relPath, writeID)
}

func (s *Service) createFileLocked(relPath, writeID string) error {
	p, err := s.currentProject()
	if err != nil {
		return err
	}
	rel, err := normalizeRel(relPath)
	if err != nil {
		return err
	}
	abs, err := resolveIn(p, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("已存在: %s", rel)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_ = f.Close()
	s.registerWrite(p, writeID, []string{rel})
	return nil
}

// Rename 同级改名。
func (s *Service) Rename(relPath, newName, writeID string) (MutationResult, error) {
	rel, err := normalizeRel(relPath)
	if err != nil {
		return MutationResult{}, err
	}
	name := strings.TrimSpace(newName)
	if strings.ContainsAny(name, `/\`) {
		// Agent 常把完整路径写进 new_name；同级改名只需 basename。
		name = filepath.Base(filepath.FromSlash(name))
	}
	if name == "" || strings.ContainsAny(name, `/\`) {
		return MutationResult{}, fmt.Errorf("无效名称：new_name 只要文件名（如 第三章_兰若寺.md），不要写 章节/… 路径")
	}
	parent := filepath.ToSlash(filepath.Dir(rel))
	var to string
	if parent == "." || parent == "" {
		to = name
	} else {
		to = parent + "/" + name
	}
	return s.Move(rel, to, writeID)
}

// Move 移动文件或目录；目标已存在则报错；禁止移入自身子树。
func (s *Service) Move(fromRel, toRel, writeID string) (MutationResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.moveLocked(fromRel, toRel, writeID)
}

func (s *Service) moveLocked(fromRel, toRel, writeID string) (MutationResult, error) {
	p, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	from, err := normalizeRel(fromRel)
	if err != nil {
		return MutationResult{}, err
	}
	to, err := normalizeRel(toRel)
	if err != nil {
		return MutationResult{}, err
	}
	if from == to {
		return MutationResult{OK: []string{from}, MovedTo: map[string]string{from: to}}, nil
	}
	if isPathInside(from, to) {
		return MutationResult{}, fmt.Errorf("不能移动到自身或子目录内")
	}
	srcAbs, err := resolveIn(p, from)
	if err != nil {
		return MutationResult{}, err
	}
	dstAbs, err := resolveIn(p, to)
	if err != nil {
		return MutationResult{}, err
	}
	if _, err := os.Stat(srcAbs); err != nil {
		return MutationResult{}, err
	}
	if _, err := os.Stat(dstAbs); err == nil {
		return MutationResult{}, fmt.Errorf("目标已存在: %s", to)
	} else if !os.IsNotExist(err) {
		return MutationResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return MutationResult{}, err
	}
	if err := os.Rename(srcAbs, dstAbs); err != nil {
		if err2 := copyPath(srcAbs, dstAbs); err2 != nil {
			return MutationResult{}, err
		}
		if err2 := os.RemoveAll(srcAbs); err2 != nil {
			return MutationResult{}, err2
		}
	}
	s.registerWrite(p, writeID, []string{from, to})
	return MutationResult{
		OK:      []string{from},
		MovedTo: map[string]string{from: to},
	}, nil
}

// Copy 复制文件或目录；目标已存在则报错。
func (s *Service) Copy(fromRel, toRel, writeID string) (MutationResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	p, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	from, err := normalizeRel(fromRel)
	if err != nil {
		return MutationResult{}, err
	}
	to, err := normalizeRel(toRel)
	if err != nil {
		return MutationResult{}, err
	}
	if from == to {
		return MutationResult{}, fmt.Errorf("源与目标相同")
	}
	if isPathInside(from, to) {
		return MutationResult{}, fmt.Errorf("不能复制到自身或子目录内")
	}
	srcAbs, err := resolveIn(p, from)
	if err != nil {
		return MutationResult{}, err
	}
	dstAbs, err := resolveIn(p, to)
	if err != nil {
		return MutationResult{}, err
	}
	if _, err := os.Stat(srcAbs); err != nil {
		return MutationResult{}, err
	}
	if _, err := os.Stat(dstAbs); err == nil {
		return MutationResult{}, fmt.Errorf("目标已存在: %s", to)
	} else if !os.IsNotExist(err) {
		return MutationResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o755); err != nil {
		return MutationResult{}, err
	}
	if err := copyPath(srcAbs, dstAbs); err != nil {
		return MutationResult{}, err
	}
	s.registerWrite(p, writeID, []string{to})
	return MutationResult{OK: []string{to}}, nil
}

// Delete 删除文件或目录。
func (s *Service) Delete(relPath, writeID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.deleteLocked(relPath, writeID)
}

func (s *Service) deleteLocked(relPath, writeID string) error {
	p, err := s.currentProject()
	if err != nil {
		return err
	}
	rel, err := normalizeRel(relPath)
	if err != nil {
		return err
	}
	abs, err := resolveIn(p, rel)
	if err != nil {
		return err
	}
	rootClean, _ := filepath.Abs(p.Root)
	if abs == rootClean {
		return fmt.Errorf("不能删除项目根")
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if err := os.RemoveAll(abs); err != nil {
			return err
		}
	} else if err := os.Remove(abs); err != nil {
		return err
	}
	s.registerWrite(p, writeID, []string{rel})
	return nil
}

// MinimalDeleteRoots 父路径已选则丢弃子路径。
func MinimalDeleteRoots(rels []string) []string {
	norm := make([]string, 0, len(rels))
	seen := map[string]struct{}{}
	for _, r := range rels {
		r, err := normalizeRel(r)
		if err != nil {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		norm = append(norm, r)
	}
	out := make([]string, 0, len(norm))
	for _, r := range norm {
		covered := false
		for _, other := range norm {
			if other != r && isPathInside(other, r) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, r)
		}
	}
	return out
}

// BatchDelete 批量删除；先压成最小删除根。
func (s *Service) BatchDelete(rels []string, writeID string) MutationResult {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	roots := MinimalDeleteRoots(rels)
	res := MutationResult{Failed: map[string]string{}}
	touched := make([]string, 0, len(roots))
	for _, r := range roots {
		if err := s.deleteLocked(r, ""); err != nil {
			res.Failed[r] = err.Error()
		} else {
			res.OK = append(res.OK, r)
			touched = append(touched, r)
		}
	}
	if len(res.Failed) == 0 {
		res.Failed = nil
	}
	if p, err := s.currentProject(); err == nil {
		s.registerWrite(p, writeID, touched)
	}
	return res
}

// BatchMove 将多项移入 destDir；重名自动避让。destDir 空串表示项目根。
func (s *Service) BatchMove(rels []string, destDir, writeID string) MutationResult {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res := MutationResult{
		Failed:  map[string]string{},
		MovedTo: map[string]string{},
	}
	p, err := s.currentProject()
	if err != nil {
		for _, r := range rels {
			res.Failed[r] = err.Error()
		}
		return res
	}

	destParent := strings.Trim(filepath.ToSlash(strings.TrimSpace(destDir)), "/")
	var parentAbs string
	if destParent == "" {
		parentAbs = p.Root
	} else {
		parentAbs, err = resolveIn(p, destParent)
		if err != nil {
			for _, r := range rels {
				res.Failed[r] = err.Error()
			}
			return res
		}
		st, err := os.Stat(parentAbs)
		if err != nil {
			for _, r := range rels {
				res.Failed[r] = err.Error()
			}
			return res
		}
		if !st.IsDir() {
			for _, r := range rels {
				res.Failed[r] = "目标不是目录"
			}
			return res
		}
	}

	roots := MinimalDeleteRoots(rels)
	touched := make([]string, 0, len(roots)*2)
	for _, from := range roots {
		base := filepath.Base(from)
		name := uniqueDestName(parentAbs, base)
		var to string
		if destParent == "" {
			to = name
		} else {
			to = destParent + "/" + name
		}
		if isPathInside(from, to) {
			res.Failed[from] = "不能移动到自身或子目录内"
			continue
		}
		m, err := s.moveLocked(from, to, "")
		if err != nil {
			res.Failed[from] = err.Error()
			continue
		}
		res.OK = append(res.OK, from)
		for k, v := range m.MovedTo {
			res.MovedTo[k] = v
			touched = append(touched, k, v)
		}
	}
	if len(res.Failed) == 0 {
		res.Failed = nil
	}
	if len(res.MovedTo) == 0 {
		res.MovedTo = nil
	}
	s.registerWrite(p, writeID, touched)
	return res
}

// ImportExternal 将项目外（或项目内）绝对路径导入 destDir。
// 源在项目内则移动；源在项目外则复制；重名自动避让。
func (s *Service) ImportExternal(absPaths []string, destDir, writeID string) MutationResult {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res := MutationResult{
		Failed:  map[string]string{},
		MovedTo: map[string]string{},
	}
	p, err := s.currentProject()
	if err != nil {
		for _, a := range absPaths {
			res.Failed[a] = err.Error()
		}
		return res
	}
	destParent := strings.Trim(filepath.ToSlash(strings.TrimSpace(destDir)), "/")
	var parentAbs string
	if destParent == "" {
		parentAbs = p.Root
	} else {
		parentAbs, err = resolveIn(p, destParent)
		if err != nil {
			for _, a := range absPaths {
				res.Failed[a] = err.Error()
			}
			return res
		}
		st, err := os.Stat(parentAbs)
		if err != nil || !st.IsDir() {
			msg := "目标不是目录"
			if err != nil {
				msg = err.Error()
			}
			for _, a := range absPaths {
				res.Failed[a] = msg
			}
			return res
		}
	}

	rootClean, _ := filepath.Abs(p.Root)
	touched := make([]string, 0, len(absPaths)*2)
	for _, raw := range absPaths {
		srcAbs, err := filepath.Abs(strings.TrimSpace(raw))
		if err != nil {
			res.Failed[raw] = err.Error()
			continue
		}
		if _, err := os.Stat(srcAbs); err != nil {
			res.Failed[raw] = err.Error()
			continue
		}
		base := filepath.Base(srcAbs)
		name := uniqueDestName(parentAbs, base)
		var toRel string
		if destParent == "" {
			toRel = name
		} else {
			toRel = destParent + "/" + name
		}
		dstAbs, err := resolveIn(p, toRel)
		if err != nil {
			res.Failed[raw] = err.Error()
			continue
		}

		// 源已在项目内：不经「访达导入」误搬（树内拖会走 BatchMove）；跳过以免拖出副本/本机路径
		if srcAbs == rootClean || strings.HasPrefix(srcAbs, rootClean+string(os.PathSeparator)) {
			continue
		}

		if err := copyPath(srcAbs, dstAbs); err != nil {
			res.Failed[raw] = err.Error()
			continue
		}
		res.OK = append(res.OK, toRel)
		touched = append(touched, toRel)
	}
	if len(res.Failed) == 0 {
		res.Failed = nil
	}
	if len(res.MovedTo) == 0 {
		res.MovedTo = nil
	}
	s.registerWrite(p, writeID, touched)
	return res
}

func copyPath(src, dst string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst, st.Mode())
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		from := filepath.Join(src, e.Name())
		to := filepath.Join(dst, e.Name())
		if err := copyPath(from, to); err != nil {
			return err
		}
	}
	return nil
}

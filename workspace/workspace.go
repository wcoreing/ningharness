package workspace

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ningharness/protocol"
	
	
)

// Project 已打开的本地目录项目。
type Project struct {
	ID   string
	Root string
}

// TreeListing ListTree 结果。
type TreeListing struct {
	Nodes     []protocol.TreeNode `json:"nodes"`
	Truncated bool                `json:"truncated"` // 保留字段；恒为 false（已取消截断）
}

// Service 无业务分区的工作区 I/O（UI 与 MCP 可共享，须加锁）。
type Service struct {
	mu      sync.RWMutex
	writeMu sync.Mutex // 串行化写盘，避免同 path 并发互相覆盖
	project *Project
}

func New() *Service {
	return &Service{}
}

func (s *Service) Current() *Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.project
}

// Open 将绝对路径目录设为当前项目。
func (s *Service) Open(absRoot string) (*Project, error) {
	root := filepath.Clean(strings.TrimSpace(absRoot))
	if root == "" || root == "." {
		return nil, fmt.Errorf("empty project path")
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	p := &Project{ID: abs, Root: abs}
	s.mu.Lock()
	s.project = p
	s.mu.Unlock()
	return p, nil
}

// ListTree 返回项目文件树（跳过 .git / node_modules 等噪点，不截断深度与条目）。
func (s *Service) ListTree() (TreeListing, error) {
	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	if p == nil {
		return TreeListing{}, fmt.Errorf("no project open")
	}
	return ListTreeAt(p.Root)
}

// ListTreeAt 列出任意绝对目录下的文件树。
func ListTreeAt(absRoot string) (TreeListing, error) {
	root := filepath.Clean(strings.TrimSpace(absRoot))
	if root == "" {
		return TreeListing{}, fmt.Errorf("empty root")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return TreeListing{}, err
	}
	out := make([]protocol.TreeNode, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if shouldSkip(name) {
			continue
		}
		node, err := buildNode(root, name, e.IsDir())
		if err != nil {
			continue
		}
		out = append(out, node)
	}
	sortTreeChildren(out)
	return TreeListing{Nodes: out}, nil
}

func buildNode(root, rel string, isDir bool) (protocol.TreeNode, error) {
	name := filepath.Base(rel)
	n := protocol.TreeNode{
		RelPath: filepath.ToSlash(rel),
		Name:    name,
		IsDir:   isDir,
	}
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if !isDir {
		n.WordCount = fileWordCount(abs, rel)
		return n, nil
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return n, err
	}
	sum := 0
	for _, e := range entries {
		childName := e.Name()
		if shouldSkip(childName) {
			continue
		}
		childRel := filepath.ToSlash(filepath.Join(rel, childName))
		child, err := buildNode(root, childRel, e.IsDir())
		if err != nil {
			continue
		}
		n.Children = append(n.Children, child)
		sum += child.WordCount
	}
	sortTreeChildren(n.Children)
	n.WordCount = sum
	return n, nil
}

const maxTreeWordBytes = 2 << 20 // 与 ReadText 上限一致，防 ListTree 卡死

// fileWordCount 文本稿面字数（docwords，去 MD 壳）；非文本 / 超大 / 含 NUL 返回 0。
func fileWordCount(abs, rel string) int {
	if IsNonTextRel(rel) {
		return 0
	}
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() || st.Size() > maxTreeWordBytes {
		return 0
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return 0
	}
	n := len(b)
	if n > 8000 {
		n = 8000
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return 0
		}
	}
	return Count(string(b))
}

func shouldSkip(name string) bool {
	switch name {
	case ".git", "node_modules", "dist", ".DS_Store", "frontend/dist":
		return true
	}
	return strings.HasPrefix(name, ".")
}

var nonTextExt = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".gif": {}, ".webp": {}, ".bmp": {}, ".ico": {},
	".pdf": {}, ".zip": {}, ".gz": {}, ".tar": {}, ".tgz": {}, ".7z": {}, ".rar": {},
	".woff": {}, ".woff2": {}, ".ttf": {}, ".otf": {}, ".eot": {},
	".mp3": {}, ".mp4": {}, ".mov": {}, ".wav": {}, ".webm": {},
	".exe": {}, ".dll": {}, ".so": {}, ".dylib": {}, ".bin": {},
	".doc": {}, ".docx": {}, ".xls": {}, ".xlsx": {}, ".ppt": {}, ".pptx": {},
}

// IsNonTextRel 按扩展名判断是否暂不支持预览。
func IsNonTextRel(relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	_, ok := nonTextExt[ext]
	return ok
}

func resolveIn(p *Project, relPath string) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no project open")
	}
	rel := filepath.Clean(filepath.FromSlash(strings.TrimSpace(relPath)))
	if rel == "." || rel == "" {
		return "", fmt.Errorf("empty relPath")
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path escapes project: %s", relPath)
	}
	abs := filepath.Join(p.Root, rel)
	absClean, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rootClean, err := filepath.Abs(p.Root)
	if err != nil {
		return "", err
	}
	if absClean != rootClean && !strings.HasPrefix(absClean, rootClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes project: %s", relPath)
	}
	return absClean, nil
}

// Resolve 将 relPath 解析为项目内绝对路径。
func (s *Service) Resolve(relPath string) (string, error) {
	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	return resolveIn(p, relPath)
}

// ReadText 读取文本文件。
func (s *Service) ReadText(relPath string) (string, error) {
	if IsNonTextRel(relPath) {
		return "", fmt.Errorf("非文本文件，暂不支持预览")
	}
	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	abs, err := resolveIn(p, relPath)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("is a directory: %s", relPath)
	}
	if st.Size() > 2<<20 {
		return "", fmt.Errorf("file too large (>2MB): %s", relPath)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// imageExtMIME Markdown 预览可内嵌的图片类型。
var imageExtMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
}

// sniffImageMIME 按文件头识别真实图片类型（生成图常误标 .png）。
func sniffImageMIME(b []byte, fallbackExt string) (string, bool) {
	if len(b) >= 8 && b[0] == 0x89 && string(b[1:4]) == "PNG" {
		return "image/png", true
	}
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return "image/jpeg", true
	}
	if len(b) >= 6 && (string(b[0:6]) == "GIF87a" || string(b[0:6]) == "GIF89a") {
		return "image/gif", true
	}
	if len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return "image/webp", true
	}
	if len(b) >= 2 && b[0] == 0x42 && b[1] == 0x4d {
		return "image/bmp", true
	}
	if mime, ok := imageExtMIME[strings.ToLower(fallbackExt)]; ok {
		return mime, true
	}
	return "", false
}

// ReadDataURL 读项目内图片为 data URL（MD / 中栏预览用）；非图片或不存在则报错。
func (s *Service) ReadDataURL(relPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(relPath))
	if _, ok := imageExtMIME[ext]; !ok {
		return "", fmt.Errorf("not an image: %s", relPath)
	}
	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	abs, err := resolveIn(p, relPath)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("is a directory: %s", relPath)
	}
	// 预览内嵌略宽于文本上限
	if st.Size() > 8<<20 {
		return "", fmt.Errorf("image too large (>8MB): %s", relPath)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	mime, ok := sniffImageMIME(b, ext)
	if !ok {
		return "", fmt.Errorf("not an image: %s", relPath)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// WriteText 写入文本；父目录不存在则创建。非空 writeId 登记供 fsnotify 解析来源。
// 同 Service 内写盘串行，降低 UI / Agent / MCP 同 path 竞态丢更新。
func (s *Service) WriteText(relPath, content, writeID string) error {
	return s.WriteBytes(relPath, []byte(content), writeID)
}

// WriteBytes 写入二进制（如 PNG）；父目录不存在则创建。非空 writeId 登记供 fsnotify。
func (s *Service) WriteBytes(relPath string, data []byte, writeID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	abs, err := resolveIn(p, relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if p != nil && strings.TrimSpace(writeID) != "" {
		rel, err := filepath.Rel(p.Root, abs)
		if err == nil {
			Register(p.ID, writeID, []string{filepath.ToSlash(rel)})
		}
	}
	return os.WriteFile(abs, data, 0o644)
}

package workspace

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	defaultGrepMaxMatches = 40
	maxGrepMaxMatches     = 200
	maxGrepFileBytes      = 2 << 20 // 与 ReadText 一致
	maxGrepLineRunes      = 400
)

// GrepOpts 项目内内容搜索选项。
type GrepOpts struct {
	Pattern         string
	Path            string // 相对路径：文件或目录；空=项目根
	Glob            string // 如 *.md；空=全部文本
	CaseInsensitive bool
	Regex           bool // false=字面量；true=Go regexp
	MaxMatches      int
}

// GrepHit 单行命中。
type GrepHit struct {
	RelPath string
	Line    int
	Text    string
}

// Grep 在当前项目内搜索文本；跳过与 ListTree 相同的噪点目录与非文本扩展名。
func (s *Service) Grep(opts GrepOpts) ([]GrepHit, error) {
	s.mu.RLock()
	p := s.project
	s.mu.RUnlock()
	if p == nil {
		return nil, fmt.Errorf("no project open")
	}
	pat := strings.TrimSpace(opts.Pattern)
	if pat == "" {
		return nil, fmt.Errorf("pattern required")
	}
	max := opts.MaxMatches
	if max <= 0 {
		max = defaultGrepMaxMatches
	}
	if max > maxGrepMaxMatches {
		max = maxGrepMaxMatches
	}
	re, err := compileGrepPattern(pat, opts.Regex, opts.CaseInsensitive)
	if err != nil {
		return nil, err
	}
	startRel := filepath.ToSlash(strings.TrimSpace(opts.Path))
	startAbs := p.Root
	if startRel != "" && startRel != "." {
		abs, rerr := resolveIn(p, startRel)
		if rerr != nil {
			return nil, rerr
		}
		startAbs = abs
	}
	st, err := os.Stat(startAbs)
	if err != nil {
		return nil, err
	}
	glob := strings.TrimSpace(opts.Glob)
	var hits []GrepHit
	truncated := false
	add := func(h GrepHit) bool {
		hits = append(hits, h)
		if len(hits) >= max {
			truncated = true
			return false
		}
		return true
	}
	if !st.IsDir() {
		rel, rerr := relFromRoot(p.Root, startAbs)
		if rerr != nil {
			return nil, rerr
		}
		if !grepGlobOK(rel, glob) || IsNonTextRel(rel) {
			return hits, nil
		}
		_ = grepFile(startAbs, rel, re, add)
		return hits, nil
	}
	err = filepath.Walk(startAbs, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := info.Name()
		if path != startAbs && shouldSkip(name) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, rerr := relFromRoot(p.Root, path)
		if rerr != nil {
			return nil
		}
		if IsNonTextRel(rel) || !grepGlobOK(rel, glob) {
			return nil
		}
		if !grepFile(path, rel, re, add) {
			return fmt.Errorf("grep: truncated")
		}
		return nil
	})
	if err != nil && !truncated {
		return hits, err
	}
	_ = truncated
	return hits, nil
}

func compileGrepPattern(pat string, asRegex, ci bool) (*regexp.Regexp, error) {
	expr := pat
	if !asRegex {
		expr = regexp.QuoteMeta(pat)
	}
	if ci {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}
	return re, nil
}

func relFromRoot(root, abs string) (string, error) {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path escapes project")
	}
	return rel, nil
}

func grepGlobOK(rel, glob string) bool {
	if glob == "" {
		return true
	}
	base := filepath.Base(rel)
	ok, err := filepath.Match(glob, base)
	if err == nil && ok {
		return true
	}
	ok, err = filepath.Match(glob, rel)
	return err == nil && ok
}

func grepFile(abs, rel string, re *regexp.Regexp, add func(GrepHit) bool) bool {
	st, err := os.Stat(abs)
	if err != nil || st.IsDir() || st.Size() > maxGrepFileBytes {
		return true
	}
	f, err := os.Open(abs)
	if err != nil {
		return true
	}
	defer f.Close()
	// 快速跳过含 NUL 的头
	head := make([]byte, 8000)
	n, _ := f.Read(head)
	for i := 0; i < n; i++ {
		if head[i] == 0 {
			return true
		}
	}
	if _, err := f.Seek(0, 0); err != nil {
		return true
	}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if !re.MatchString(line) {
			continue
		}
		text := line
		if utf8.RuneCountInString(text) > maxGrepLineRunes {
			runes := []rune(text)
			text = string(runes[:maxGrepLineRunes]) + "…"
		}
		if !add(GrepHit{RelPath: rel, Line: lineNo, Text: text}) {
			return false
		}
	}
	return true
}

// FormatGrepHits 工具回执文本。
func FormatGrepHits(pattern string, hits []GrepHit, max int) string {
	if max <= 0 {
		max = defaultGrepMaxMatches
	}
	if len(hits) == 0 {
		return fmt.Sprintf("grep %q：无命中", pattern)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "grep %q：%d 命中", pattern, len(hits))
	if len(hits) >= max {
		fmt.Fprintf(&b, "（已截断，收窄 path/glob 或提高 max_matches）")
	}
	b.WriteByte('\n')
	for _, h := range hits {
		fmt.Fprintf(&b, "%s:%d:%s\n", h.RelPath, h.Line, h.Text)
	}
	return strings.TrimRight(b.String(), "\n")
}

package skill

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

type packMeta struct {
	Slug    string `json:"slug"`
	Version string `json:"version"`
}

// ImportZip 将 Cursor/市场风格 Skill 包（zip 内含 SKILL.md）导入为 skills/<id>/。
// idOverride 空则取 frontmatter name → _meta.slug → zip 文件名。
// overwrite=false 且目标已存在则报错。
// 返回 Info 与写入的相对路径列表（供雷达）。
func ImportZip(projectRoot, zipPath, idOverride string, overwrite bool) (Info, []string, error) {
	root := strings.TrimSpace(projectRoot)
	zipPath = filepath.Clean(strings.TrimSpace(zipPath))
	if root == "" {
		return Info{}, nil, fmt.Errorf("empty project root")
	}
	if zipPath == "" {
		return Info{}, nil, fmt.Errorf("empty zip path")
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return Info{}, nil, err
	}
	defer zr.Close()

	prefix, err := detectSkillZipPrefix(zr.File)
	if err != nil {
		return Info{}, nil, err
	}

	skillRaw, err := readZipFile(zr, prefix+"SKILL.md")
	if err != nil {
		return Info{}, nil, fmt.Errorf("read SKILL.md: %w", err)
	}
	fm, _, err := parseSkillMD(string(skillRaw))
	if err != nil {
		return Info{}, nil, fmt.Errorf("SKILL.md frontmatter: %w", err)
	}

	id := strings.TrimSpace(idOverride)
	if id == "" {
		id = sanitizeID(fm.Name)
	}
	if id == "" || !idRe.MatchString(id) {
		metaRaw, _ := readZipFile(zr, prefix+"_meta.json")
		var meta packMeta
		_ = json.Unmarshal(metaRaw, &meta)
		id = sanitizeID(meta.Slug)
	}
	if id == "" || !idRe.MatchString(id) {
		base := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
		id = sanitizeID(base)
	}
	if !idRe.MatchString(id) {
		return Info{}, nil, fmt.Errorf("cannot derive valid skill id from package (got %q)", id)
	}

	destDir := filepath.Join(Dir(root), id)
	if st, err := os.Stat(destDir); err == nil && st.IsDir() && !overwrite {
		return Info{}, nil, fmt.Errorf("skill already exists: %s (pass overwrite)", id)
	}
	if overwrite {
		_ = os.RemoveAll(destDir)
	}

	written := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		name := filepath.ToSlash(f.Name)
		if shouldSkipZipEntry(name) {
			continue
		}
		relInPack := strings.TrimPrefix(name, prefix)
		if relInPack == "" || strings.HasSuffix(relInPack, "/") {
			continue
		}
		if strings.Contains(relInPack, "..") {
			continue
		}
		rel := filepath.ToSlash(filepath.Join(RootDir, id, relInPack))
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(abs, 0o755); err != nil {
				return Info{}, nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return Info{}, nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return Info{}, nil, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, 8<<20))
		_ = rc.Close()
		if err != nil {
			return Info{}, nil, err
		}
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			return Info{}, nil, err
		}
		written = append(written, rel)
	}

	name := strings.TrimSpace(fm.Name)
	if name == "" {
		name = id
	}
	desc := strings.TrimSpace(fm.Description)
	if desc == "" {
		desc = name
	}
	info := Info{
		ID:          id,
		Name:        name,
		Description: desc,
		RelDir:      filepath.ToSlash(filepath.Join(RootDir, id)),
	}
	if _, err := os.Stat(filepath.Join(destDir, LessonsFile)); err == nil {
		info.HasLessons = true
	}
	return info, written, nil
}

func detectSkillZipPrefix(files []*zip.File) (string, error) {
	var names []string
	for _, f := range files {
		n := filepath.ToSlash(f.Name)
		if shouldSkipZipEntry(n) {
			continue
		}
		names = append(names, n)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("empty zip")
	}
	for _, n := range names {
		if n == "SKILL.md" {
			return "", nil
		}
	}
	// 单层包裹目录：foo/SKILL.md
	var top string
	for _, n := range names {
		parts := strings.Split(n, "/")
		if len(parts) < 2 {
			continue
		}
		if top == "" {
			top = parts[0]
		} else if top != parts[0] {
			return "", fmt.Errorf("zip has multiple roots; need SKILL.md at root or single folder")
		}
	}
	if top == "" {
		return "", fmt.Errorf("SKILL.md not found in zip")
	}
	want := top + "/SKILL.md"
	for _, n := range names {
		if n == want {
			return top + "/", nil
		}
	}
	return "", fmt.Errorf("SKILL.md not found in zip")
}

func readZipFile(zr *zip.ReadCloser, name string) ([]byte, error) {
	name = filepath.ToSlash(name)
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 4<<20))
		}
	}
	return nil, fmt.Errorf("%s not found", name)
}

func shouldSkipZipEntry(name string) bool {
	n := filepath.ToSlash(name)
	if n == "" || strings.HasPrefix(n, "__MACOSX/") || strings.Contains(n, "/__MACOSX/") {
		return true
	}
	base := filepath.Base(n)
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")
	if len(s) > 64 {
		s = s[:64]
		s = strings.Trim(s, "-_")
	}
	if s == "" {
		return ""
	}
	// 须字母或数字开头
	if s[0] == '-' || s[0] == '_' {
		s = "s" + s
	}
	return s
}

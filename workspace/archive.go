package workspace

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxUnzipBytes int64 = 512 << 20 // 512MB 解压总量上限，防 zip bomb
	maxUnzipFiles       = 10000
)

// ZipPaths 将选中路径压成同级 zip；名自动生成，重名避让。
// 单项 → {basename}.zip；多项 → 归档.zip。OK 含生成的 zip 相对路径。
func (s *Service) ZipPaths(rels []string, writeID string) MutationResult {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	res := MutationResult{Failed: map[string]string{}}
	p, err := s.currentProject()
	if err != nil {
		for _, r := range rels {
			res.Failed[r] = err.Error()
		}
		return res
	}

	roots := MinimalDeleteRoots(rels)
	if len(roots) == 0 {
		res.Failed["_"] = "请先选择文件或文件夹"
		return res
	}

	parentRel := dirnameRel(roots[0])
	for _, r := range roots[1:] {
		if dirnameRel(r) != parentRel {
			res.Failed[r] = "请选择同一文件夹下的项再压缩"
			return finalizeFailed(res)
		}
	}

	var parentAbs string
	if parentRel == "" {
		parentAbs = p.Root
	} else {
		parentAbs, err = resolveIn(p, parentRel)
		if err != nil {
			res.Failed[roots[0]] = err.Error()
			return finalizeFailed(res)
		}
	}

	zipBase := autoZipName(roots)
	zipName := uniqueDestName(parentAbs, zipBase)
	var zipRel string
	if parentRel == "" {
		zipRel = zipName
	} else {
		zipRel = parentRel + "/" + zipName
	}
	zipAbs, err := resolveIn(p, zipRel)
	if err != nil {
		res.Failed[roots[0]] = err.Error()
		return finalizeFailed(res)
	}

	if err := writeZip(p, roots, zipAbs); err != nil {
		_ = os.Remove(zipAbs)
		res.Failed[roots[0]] = err.Error()
		return finalizeFailed(res)
	}

	s.registerWrite(p, writeID, []string{zipRel})
	return MutationResult{OK: []string{zipRel}}
}

// UnzipPath 解压 zip 到同级自动命名目录（{stem}/，重名避让）。OK 含解出目录。
func (s *Service) UnzipPath(relPath, writeID string) (MutationResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	p, err := s.currentProject()
	if err != nil {
		return MutationResult{}, err
	}
	rel, err := normalizeRel(relPath)
	if err != nil {
		return MutationResult{}, err
	}
	if !isZipRel(rel) {
		return MutationResult{}, fmt.Errorf("仅支持 .zip：%s", rel)
	}
	srcAbs, err := resolveIn(p, rel)
	if err != nil {
		return MutationResult{}, err
	}
	st, err := os.Stat(srcAbs)
	if err != nil {
		return MutationResult{}, err
	}
	if st.IsDir() {
		return MutationResult{}, fmt.Errorf("不是 zip 文件: %s", rel)
	}

	parentRel := dirnameRel(rel)
	var parentAbs string
	if parentRel == "" {
		parentAbs = p.Root
	} else {
		parentAbs, err = resolveIn(p, parentRel)
		if err != nil {
			return MutationResult{}, err
		}
	}

	stem := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	if stem == "" {
		stem = "解压"
	}
	dirName := uniqueDestName(parentAbs, stem)
	var destRel string
	if parentRel == "" {
		destRel = dirName
	} else {
		destRel = parentRel + "/" + dirName
	}
	destAbs, err := resolveIn(p, destRel)
	if err != nil {
		return MutationResult{}, err
	}
	if err := os.MkdirAll(destAbs, 0o755); err != nil {
		return MutationResult{}, err
	}

	if err := extractZip(srcAbs, destAbs); err != nil {
		_ = os.RemoveAll(destAbs)
		return MutationResult{}, err
	}

	s.registerWrite(p, writeID, []string{destRel})
	return MutationResult{OK: []string{destRel}}, nil
}

func autoZipName(roots []string) string {
	if len(roots) == 1 {
		return filepath.Base(roots[0]) + ".zip"
	}
	return "归档.zip"
}

func dirnameRel(rel string) string {
	r := filepath.ToSlash(strings.Trim(rel, "/"))
	i := strings.LastIndex(r, "/")
	if i < 0 {
		return ""
	}
	return r[:i]
}

func isZipRel(rel string) bool {
	return strings.EqualFold(filepath.Ext(rel), ".zip")
}

func finalizeFailed(res MutationResult) MutationResult {
	if len(res.Failed) == 0 {
		res.Failed = nil
	}
	return res
}

func writeZip(p *Project, roots []string, zipAbs string) error {
	f, err := os.Create(zipAbs)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, rootRel := range roots {
		abs, err := resolveIn(p, rootRel)
		if err != nil {
			return err
		}
		st, err := os.Stat(abs)
		if err != nil {
			return err
		}
		entryRoot := filepath.Base(rootRel)
		if !st.IsDir() {
			if err := addFileToZip(zw, abs, entryRoot); err != nil {
				return err
			}
			continue
		}
		err = filepath.Walk(abs, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			name := info.Name()
			if path != abs && shouldSkip(name) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			relInside, err := filepath.Rel(abs, path)
			if err != nil {
				return err
			}
			relInside = filepath.ToSlash(relInside)
			var entry string
			if relInside == "." {
				entry = entryRoot + "/"
			} else {
				entry = entryRoot + "/" + relInside
			}
			if info.IsDir() {
				if !strings.HasSuffix(entry, "/") {
					entry += "/"
				}
				_, err := zw.Create(entry)
				return err
			}
			return addFileToZip(zw, path, entry)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func addFileToZip(zw *zip.Writer, abs, entry string) error {
	w, err := zw.Create(entry)
	if err != nil {
		return err
	}
	src, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(w, src)
	return err
}

func extractZip(zipAbs, destAbs string) error {
	zr, err := zip.OpenReader(zipAbs)
	if err != nil {
		return err
	}
	defer zr.Close()

	destClean := filepath.Clean(destAbs)
	var total int64
	var files int

	for _, zf := range zr.File {
		name := filepath.ToSlash(zf.Name)
		if name == "" || shouldSkipZipEntry(name) {
			continue
		}
		// zip slip
		target := filepath.Join(destClean, filepath.FromSlash(name))
		targetClean := filepath.Clean(target)
		if targetClean != destClean && !strings.HasPrefix(targetClean, destClean+string(os.PathSeparator)) {
			return fmt.Errorf("非法路径: %s", name)
		}

		if zf.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(targetClean, 0o755); err != nil {
				return err
			}
			continue
		}

		files++
		if files > maxUnzipFiles {
			return fmt.Errorf("解压文件数超过上限（%d）", maxUnzipFiles)
		}
		if err := os.MkdirAll(filepath.Dir(targetClean), 0o755); err != nil {
			return err
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(targetClean, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxUnzipBytes-total+1))
		_ = out.Close()
		_ = rc.Close()
		if err != nil {
			return err
		}
		total += n
		if total > maxUnzipBytes {
			return fmt.Errorf("解压体积超过上限（512MB）")
		}
	}
	return nil
}

func shouldSkipZipEntry(name string) bool {
	n := filepath.ToSlash(name)
	if strings.HasPrefix(n, "__MACOSX/") || n == "__MACOSX" {
		return true
	}
	base := filepath.Base(n)
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

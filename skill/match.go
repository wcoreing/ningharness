package skill

import (
	"path/filepath"
	"strings"
)

// MatchForPaths 返回 globs 命中任一相对路径的 Skill（无 globs 的不参与路径挂载）。
func MatchForPaths(projectRoot string, rels []string) []Info {
	list, err := List(projectRoot)
	if err != nil || len(list) == 0 {
		return nil
	}
	var paths []string
	for _, r := range rels {
		r = filepath.ToSlash(strings.TrimSpace(r))
		if r != "" {
			paths = append(paths, r)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	var out []Info
	for _, info := range list {
		if !info.Enabled || len(info.Globs) == 0 {
			continue
		}
		for _, g := range info.Globs {
			if matchAnyPath(g, paths) {
				out = append(out, info)
				break
			}
		}
	}
	return out
}

func matchAnyPath(pattern string, paths []string) bool {
	for _, p := range paths {
		if MatchPath(pattern, p) {
			return true
		}
	}
	return false
}

// MatchPath 轻量 glob：支持精确、前缀/**、以及 filepath.Match（* ?）。
func MatchPath(pattern, rel string) bool {
	p := filepath.ToSlash(strings.TrimSpace(pattern))
	r := filepath.ToSlash(strings.TrimSpace(rel))
	if p == "" || r == "" {
		return false
	}
	if strings.HasSuffix(p, "/**") {
		pre := strings.TrimSuffix(p, "/**")
		return r == pre || strings.HasPrefix(r, pre+"/")
	}
	if strings.HasPrefix(p, "**/") {
		suf := strings.TrimPrefix(p, "**/")
		if strings.Contains(suf, "*") {
			ok, _ := filepath.Match(suf, filepath.Base(r))
			if ok {
				return true
			}
			// **/a/*/b.md style — try match on full path with * only
			ok, _ = filepath.Match(suf, r)
			return ok || strings.HasSuffix(r, "/"+suf) || r == suf
		}
		return r == suf || strings.HasSuffix(r, "/"+suf)
	}
	if strings.ContainsAny(p, "*?") {
		ok, _ := filepath.Match(p, r)
		if ok {
			return true
		}
		ok, _ = filepath.Match(p, filepath.Base(r))
		return ok
	}
	return r == p || strings.HasPrefix(r, p+"/")
}

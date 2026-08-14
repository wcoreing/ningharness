// Package metadir 项目与家目录元数据路径（统一 .ningharness）。
package metadir

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// Name 项目元数据目录名（traces / goals / truncated / 测试库等）。
	Name = ".ningharness"
	// HomeName 用户家目录下台面库目录（~/.ningharness）。
	HomeName = ".ningharness"
)

// ProjectDir 项目下元数据目录（root/.ningharness）。
func ProjectDir(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return Name
	}
	return filepath.Join(root, Name)
}

// ProjectRel 相对项目根的元数据目录名。
func ProjectRel(root string) string {
	_ = root
	return Name
}

// JoinProject 拼项目元数据子路径（绝对）。
func JoinProject(root string, elem ...string) string {
	parts := append([]string{ProjectDir(root)}, elem...)
	return filepath.Join(parts...)
}

// HomeDir 台面库家目录 ~/.ningharness。
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, HomeName), nil
}

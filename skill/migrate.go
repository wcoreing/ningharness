package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateRootIfNeeded 将旧 skills/ 迁到 system/skills/（一次性）。
// - 仅有旧根：rename/move
// - 仅有新根或皆无：无操作
// - 两边都有：保留 system/skills，不覆盖；返回 migrated=false
func MigrateRootIfNeeded(projectRoot string) (migrated bool, err error) {
	root := strings.TrimSpace(projectRoot)
	if root == "" {
		return false, nil
	}
	legacy := filepath.Join(root, filepath.FromSlash(LegacyRootDir))
	 neo := Dir(root)
	legInfo, legErr := os.Stat(legacy)
	neoInfo, neoErr := os.Stat(neo)
	hasLeg := legErr == nil && legInfo.IsDir()
	hasNeo := neoErr == nil && neoInfo.IsDir()
	if !hasLeg {
		return false, nil
	}
	if hasNeo {
		// 两边都有：不自动合并
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(neo), 0o755); err != nil {
		return false, fmt.Errorf("skills migrate mkdir: %w", err)
	}
	if err := os.Rename(legacy, neo); err != nil {
		return false, fmt.Errorf("skills migrate rename: %w", err)
	}
	return true, nil
}

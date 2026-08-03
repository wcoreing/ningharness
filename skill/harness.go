package skill

import "strings"

// NoSilentInjectIDs 编排类 Skill：路径命中只 Surface（写前请加载），不静默 Inject LESSONS。
// 避免未 get_skill 选用却灌入与写盘互斥的流程契约（如入队勿抢写）。
var NoSilentInjectIDs = map[string]bool{
	"skill-train": true,
	"extract":     true,
}

// FilterLessonInjectIDs 从路径匹配结果中去掉编排类，供 Turn 前馈 InjectBrief 使用。
func FilterLessonInjectIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || NoSilentInjectIDs[id] {
			continue
		}
		out = append(out, id)
	}
	return out
}

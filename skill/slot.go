package skill

import "strings"

// PathsValueKey 可写入 RunState.Values，供 assemble 做路径匹配。
const PathsValueKey = "skill.paths"

// Slot 可换 Skill 插槽：列表 / 路径匹配 / 加载正文。
type Slot interface {
	List(projectRoot string) ([]Info, error)
	Match(projectRoot string, rels []string) []Info
	Load(projectRoot, id string) (Info, string, error)
}

// Disk 默认实现：读 system/skills 磁盘契约。
type Disk struct{}

// NewDisk 返回默认 Skill Slot。
func NewDisk() *Disk {
	return &Disk{}
}

func (Disk) List(projectRoot string) ([]Info, error) {
	return ListEnabled(projectRoot)
}

func (Disk) Match(projectRoot string, rels []string) []Info {
	return MatchForPaths(projectRoot, rels)
}

func (Disk) Load(projectRoot, id string) (Info, string, error) {
	return LoadBody(projectRoot, id)
}

var _ Slot = Disk{}

// PathsFromValues 从 RunState.Values 解析相对路径列表。
func PathsFromValues(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	v, ok := values[PathsValueKey]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case []string:
		return trimPaths(t)
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return trimPaths(out)
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func trimPaths(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// IDs 从 Info 列表取 id。
func IDs(list []Info) []string {
	out := make([]string, 0, len(list))
	for _, info := range list {
		id := strings.TrimSpace(info.ID)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

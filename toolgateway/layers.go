package toolgateway

import "strings"

// ToolLayer 核工具能力层：授权按层组合，避免散落工具名白名单。
type ToolLayer string

const (
	// LayerWorldRead 只读文件世界。
	LayerWorldRead ToolLayer = "world_read"
	// LayerWorldWrite 改写文件世界（含 mkdir/rename/delete 等）。
	LayerWorldWrite ToolLayer = "world_write"
	// LayerCognition 会话检索 / skill / lesson。
	LayerCognition ToolLayer = "cognition"
	// LayerControl 任务台账 / Trace / 队列调度。
	LayerControl ToolLayer = "control"
)

// coreToolLayers 核工具 → 层（未列出的产品扩展工具不在此表，须调用方显式放行）。
var coreToolLayers = map[string]ToolLayer{
	"list_tree":   LayerWorldRead,
	"read_file":   LayerWorldRead,
	"grep":        LayerWorldRead,
	"write_file":  LayerWorldWrite,
	"edit":        LayerWorldWrite,
	"mkdir":       LayerWorldWrite,
	"create_file": LayerWorldWrite,
	"rename_path": LayerWorldWrite,
	"move_path":   LayerWorldWrite,
	"copy_path":   LayerWorldWrite,
	"delete_path": LayerWorldWrite,
	"batch_delete_paths": LayerWorldWrite,
	"batch_move_paths":   LayerWorldWrite,

	"search_session":            LayerCognition,
	"recall_resource":           LayerCognition,
	"append_session_message":    LayerCognition,
	"get_session_brief":         LayerCognition,
	"list_skills":               LayerCognition,
	"get_skill":                 LayerCognition,
	"create_project_skill":      LayerCognition,
	"append_project_skill_note": LayerCognition,
	"set_lesson_status":         LayerCognition,
	"list_lessons":              LayerCognition,
	"ack_lesson":                LayerCognition,

	"list_tasks":                    LayerControl,
	"get_task_summary":              LayerControl,
	"get_task_trace":                LayerControl,
	"enqueue_agent_turn":            LayerControl,
	"enqueue_goal":                  LayerControl,
	"resume_goal":                   LayerControl,
	"get_goal_status":               LayerControl,
	"enqueue_agent_turns_for_paths": LayerControl,
	"list_queue":                    LayerControl,
	"cancel_queue_task":             LayerControl,
	"steer_queue_task":              LayerControl,
	"set_queue_paused":              LayerControl,
}

// CoreToolLayer 返回核工具所属层；未知工具返回空。
func CoreToolLayer(name string) ToolLayer {
	return coreToolLayers[strings.TrimSpace(name)]
}

// AllowLayers 按层生成白名单（供 ArmTurnInterceptor）。
func AllowLayers(layers ...ToolLayer) map[string]bool {
	want := map[ToolLayer]bool{}
	for _, l := range layers {
		want[l] = true
	}
	out := map[string]bool{}
	for name, layer := range coreToolLayers {
		if want[layer] {
			out[name] = true
		}
	}
	return out
}

// AllowAsk 只读调研：world_read + cognition（不含写盘与队列控制）。
func AllowAsk() map[string]bool {
	return AllowLayers(LayerWorldRead, LayerCognition)
}

// AllowReflect 经验沉淀：cognition + 少量只读 world。
func AllowReflect() map[string]bool {
	return AllowLayers(LayerWorldRead, LayerCognition)
}

// AllowAgent 默认可写 Agent：world 读写 + cognition + control。
func AllowAgent() map[string]bool {
	return AllowLayers(LayerWorldRead, LayerWorldWrite, LayerCognition, LayerControl)
}

// ArmLayers 按层武装本轮拦截器。
func (h *Gateway) ArmLayers(purpose string, layers ...ToolLayer) {
	h.ArmTurnInterceptor(purpose, AllowLayers(layers...))
}

// ArmPurpose 按常见 purpose 武装：ask|reflect → 收窄；空/agent/其它 → 不限制（nil）。
func (h *Gateway) ArmPurpose(purpose string) {
	switch strings.TrimSpace(purpose) {
	case "ask":
		h.ArmTurnInterceptor("ask", AllowAsk())
	case "reflect":
		h.ArmTurnInterceptor("reflect", AllowReflect())
	default:
		h.ArmTurnInterceptor(strings.TrimSpace(purpose), nil)
	}
}

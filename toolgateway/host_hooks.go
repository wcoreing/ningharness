package toolgateway

// HostHooks 产品壳注入点：UI 订阅与写盘旁路。
// 与 Turn 投影、工具 Registry 分离；嵌入 Gateway 以保持 hub.OnX= 赋值兼容。
type HostHooks struct {
	OnOpen          func(root string)
	OnSessionChange func()
	OnPathsChanged  func(writeID string, paths []string, wordCounts map[string]int)
	OnPathsMoved    func(movedTo map[string]string)
	OnWriteWorktree func(relPath, content, writeID string) error
	OnWriteBytes    func(relPath string, data []byte, writeID string) error
	// OnContextPatch 台面 ContextPatch（pin/写盘等）；供 UI 订阅，不改写本轮开场前馈。
	OnContextPatch func(patch any)
}

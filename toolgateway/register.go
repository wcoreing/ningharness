package toolgateway

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterCoreTools(s *server.MCPServer, h *Gateway) {
	// 与 CallNamedTool 共用 Registry 中的核处理器。
	if h != nil {
		h.ensureCoreHandlers()
	}
	s.AddTool(mcp.NewTool("list_tree",
		mcp.WithDescription("列出项目文件树（跳过 .git / node_modules 等）。快照已含路径索引；需要刷新或看 skills 内部时再用。"),
	), h.toolListTree)

	s.AddTool(mcp.NewTool("search_session",
		mcp.WithDescription("按关键词检索历史对话（desk.db · FTS5/LIKE）。查「上次说过什么 / 上周约定」；项目文件用 recall_project_context；工具返回全文用 recall_resource。"),
		mcp.WithString("query", mcp.Required(), mcp.Description("关键词（可空格分隔）")),
		mcp.WithNumber("limit", mcp.Description("最多命中条数，默认 12")),
		mcp.WithString("session_id", mcp.Description("限定会话 id，空=全部会话")),
	), h.toolSearchSession)

	s.AddTool(mcp.NewTool("recall_resource",
		mcp.WithDescription("查外置资源全文（resource 表：tool_call/tool_result/diff）。history 用 resource_ids 索引；需要正文时用本工具。优先 resource_id；也可 tool_call_id + phase/kind / rel_path / query。"),
		mcp.WithNumber("resource_id", mcp.Description("resource 数字 id（摘要里 resource#N）")),
		mcp.WithString("tool_call_id", mcp.Description("工具 call id")),
		mcp.WithString("rel_path", mcp.Description("相关相对路径")),
		mcp.WithString("query", mcp.Description("关键词（搜 summary/body）")),
		mcp.WithString("phase", mcp.Description("call | result | diff，可空")),
		mcp.WithString("kind", mcp.Description("tool_call | tool_result | diff，可空")),
		mcp.WithNumber("limit", mcp.Description("列表命中数，默认 12")),
	), h.toolRecallResource)

	s.AddTool(mcp.NewTool("read_file",
		mcp.WithDescription("读取项目内文本。参数示例 {\"rel_path\":\"章节/一.md\"}；勿再包一层 arguments。"),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("相对项目根路径")),
	), h.toolReadFile)

	s.AddTool(mcp.NewTool("grep",
		mcp.WithDescription(GrepToolDesc),
		mcp.WithString("pattern", mcp.Required(), mcp.Description("搜索串；默认字面量，regex=true 时为正则")),
		mcp.WithString("path", mcp.Description("限定相对路径（文件或目录）；空=整项目")),
		mcp.WithString("glob", mcp.Description("文件名 glob，如 *.md")),
		mcp.WithBoolean("case_insensitive", mcp.Description("忽略大小写，默认 false")),
		mcp.WithBoolean("regex", mcp.Description("pattern 按正则解释，默认 false（字面量）")),
		mcp.WithNumber("max_matches", mcp.Description("最多命中行，默认 40，最大 200")),
	), h.toolGrep)

	s.AddTool(mcp.NewTool("write_file",
		mcp.WithDescription(WriteFileToolDesc),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("相对项目根路径（见名知意，如 章节/第十一章.md）")),
		mcp.WithString("content", mcp.Required(), mcp.Description("写入全文（叙事正文直接原文，勿只写提纲当正文）")),
	), h.toolWriteFile)

	s.AddTool(mcp.NewTool("edit",
		mcp.WithDescription(EditToolDesc),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("相对项目根路径")),
		mcp.WithString("old_string", mcp.Required(), mcp.Description("要替换的原文（须与文件中精确一致；默认唯一）")),
		mcp.WithString("new_string", mcp.Required(), mcp.Description("替换后的文本（可空=删除该段）")),
		mcp.WithBoolean("replace_all", mcp.Description("替换全部出现；默认 false（要求唯一）")),
	), h.toolEdit)

	s.AddTool(mcp.NewTool("mkdir",
		mcp.WithDescription("创建目录（相对项目根；已存在报错）。收拾目录结构用本工具，勿用 shell。"),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("相对路径")),
	), h.toolMkdir)

	s.AddTool(mcp.NewTool("create_file",
		mcp.WithDescription("创建空文件（父目录自动建；已存在报错）。"),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("相对路径")),
	), h.toolCreateFile)

	s.AddTool(mcp.NewTool("rename_path",
		mcp.WithDescription(RenamePathToolDesc),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("原相对路径")),
		mcp.WithString("new_name", mcp.Required(), mcp.Description("新文件名/目录名（不含 /；写完整路径会自动取 basename）")),
	), h.toolRenamePath)

	s.AddTool(mcp.NewTool("move_path",
		mcp.WithDescription("移动文件或目录；目标勿已存在；禁止移入自身子树。"),
		mcp.WithString("from_rel", mcp.Required(), mcp.Description("源相对路径")),
		mcp.WithString("to_rel", mcp.Required(), mcp.Description("目标相对路径")),
	), h.toolMovePath)

	s.AddTool(mcp.NewTool("copy_path",
		mcp.WithDescription("复制文件或目录（可递归）；目标勿已存在。"),
		mcp.WithString("from_rel", mcp.Required(), mcp.Description("源相对路径")),
		mcp.WithString("to_rel", mcp.Required(), mcp.Description("目标相对路径")),
	), h.toolCopyPath)

	s.AddTool(mcp.NewTool("delete_path",
		mcp.WithDescription("删除文件或目录（目录 RemoveAll）。"),
		mcp.WithString("rel_path", mcp.Required(), mcp.Description("相对路径")),
	), h.toolDeletePath)

	s.AddTool(mcp.NewTool("batch_delete_paths",
		mcp.WithDescription("批量删除；父路径已选则自动压掉子路径。返回 ok/failed JSON。"),
		mcp.WithArray("rel_paths", mcp.Required(), mcp.Description("相对路径列表"), mcp.WithStringItems()),
	), h.toolBatchDeletePaths)

	s.AddTool(mcp.NewTool("batch_move_paths",
		mcp.WithDescription("批量移入 dest_dir（空串=项目根）；重名自动避让（副本）。返回 ok/movedTo/failed JSON。"),
		mcp.WithArray("rel_paths", mcp.Required(), mcp.Description("相对路径列表"), mcp.WithStringItems()),
		mcp.WithString("dest_dir", mcp.Description("目标目录相对路径，空=根")),
	), h.toolBatchMovePaths)

	s.AddTool(mcp.NewTool("append_session_message",
		mcp.WithDescription("往项目 active 会话（或 session_id）追加一条工作记忆消息。"),
		mcp.WithString("role", mcp.Required(), mcp.Description("user | assistant | system")),
		mcp.WithString("content", mcp.Required(), mcp.Description("正文")),
		mcp.WithString("session_id", mcp.Description("默认 active")),
	), h.toolAppendSession)

	s.AddTool(mcp.NewTool("get_session_brief",
		mcp.WithDescription("active 会话摘要：orch 键、条数、最近消息（默认含 user+assistant 各截断）。参数 users_only 只列用户原话。"),
		mcp.WithBoolean("users_only", mcp.Description("仅用户句，默认 false")),
		mcp.WithNumber("limit", mcp.Description("users_only 时最多几条 user，默认 8")),
	), h.toolSessionBrief)

	s.AddTool(mcp.NewTool("list_skills",
		mcp.WithDescription("列出项目 system/skills/*/SKILL.md（id / name / description / hasLessons）。"),
	), h.toolListSkills)

	s.AddTool(mcp.NewTool("get_skill",
		mcp.WithDescription("读取某 skill 的 SKILL.md 正文，并附 lesson_entry 经验（DB）。Cursor/外置 Agent 用；wnai 优先用 Eino skill 工具。"),
		mcp.WithString("skill", mcp.Required(), mcp.Description("skill id 或 frontmatter name")),
	), h.toolGetSkill)

	s.AddTool(mcp.NewTool("create_project_skill",
		mcp.WithDescription("新建个性化 Skill（system/skills/<id>/SKILL.md）。外置主路径：get_skill skill-author → 本工具 → get_skill 自建包。description=何时用；content=流程（仅 tools/list 工具；落盘见 write_file/edit 回执）。content 空则骨架。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("目录 id，字母数字与 _-")),
		mcp.WithString("name", mcp.Description("frontmatter name，默认=id")),
		mcp.WithString("description", mcp.Description("何时用+做什么（含触发语）")),
		mcp.WithString("content", mcp.Description("正文 markdown；空=骨架")),
	), h.toolCreateSkill)

	s.AddTool(mcp.NewTool("append_project_skill_note",
		mcp.WithDescription("写入 lesson_entry（desk.db）。scope=skill|project|personal（默认 skill）；skill scope 必填 skill；personal 跨项目；project 本项目不绑包。Host 盖章 source_task；纠正用 set_lesson_status。"),
		mcp.WithString("note", mcp.Required(), mcp.Description("经验正文")),
		mcp.WithString("scope", mcp.Description("skill|project|personal；默认 skill")),
		mcp.WithString("skill", mcp.Description("scope=skill 时必填（id 或 name）")),
		mcp.WithString("supersedes", mcp.Description("可选：被取代的 lesson id")),
	), h.toolAppendSkillNote)

	s.AddTool(mcp.NewTool("set_lesson_status",
		mcp.WithDescription("设置 lesson_entry 状态：active|superseded|expired。纠正旧经验时先对本工具再 append。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("lesson id")),
		mcp.WithString("status", mcp.Required(), mcp.Description("active|superseded|expired")),
	), h.toolSetLessonStatus)

	s.AddTool(mcp.NewTool("list_lessons",
		mcp.WithDescription("列出 lesson_entry。skill=某包全状态；scope=skill|project|personal；皆空=项目 skill active 摘要。"),
		mcp.WithString("skill", mcp.Description("skill id；优先于 scope")),
		mcp.WithString("scope", mcp.Description("skill|project|personal")),
	), h.toolListLessons)

	s.AddTool(mcp.NewTool("ack_lesson",
		mcp.WithDescription("人认账一条 lesson（acked_at）。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("lesson id")),
	), h.toolAckLesson)

	s.AddTool(mcp.NewTool("list_tasks",
		mcp.WithDescription("最近 Agent 执行台账（tasks）（默认不含 Reflect）。"),
		mcp.WithNumber("limit", mcp.Description("条数，默认 20")),
		mcp.WithBoolean("include_reflect", mcp.Description("是否含 Reflect，默认 false")),
	), h.toolListTasks)

	s.AddTool(mcp.NewTool("get_task_summary",
		mcp.WithDescription("单轮执行台账短摘要（不含工具全文/大 diff）；task_id 空=最近用户轮。全文用 recall_resource。"),
		mcp.WithString("task_id", mcp.Description("task id，可空")),
	), h.toolGetTaskSummary)

	s.AddTool(mcp.NewTool("get_task_trace",
		mcp.WithDescription("读取 Task Trace JSONL（.ningharness/traces/…）：事件流 + 恢复契约（complete / unpaired_calls）。可观测/审计。"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("task id（如 once:q-… / chat:…）")),
		mcp.WithNumber("limit", mcp.Description("最多返回末尾事件条数，默认 80")),
	), h.toolGetTaskTrace)

	s.AddTool(mcp.NewTool("enqueue_agent_turn",
		mcp.WithDescription("入队一条 agent-turn（成功回执=已入队未落盘；正文由队列节 write_file）。默认侧栏 active；session=isolated 隐藏。需 App。"),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("任务提示词")),
		mcp.WithString("driver", mcp.Description("驱动，空=默认")),
		mcp.WithString("title", mcp.Description("列表标题")),
		mcp.WithString("target_rel", mcp.Description("本轮只写路径（执行时权威注入前馈「本轮只写」；覆盖 FeedExtra 里同名旧行；入队≠落盘）")),
		mcp.WithString("session", mcp.Description("空/active=侧栏当前会话（可见）；isolated=隐藏 once:queue")),
	), h.toolEnqueueAgentTurn)

	s.AddTool(mcp.NewTool("enqueue_goal",
		mcp.WithDescription("入队 Goal 外环（type=goal）：反复跑直到 `.ningharness/goals/<id>/GOAL.yaml` status=complete|blocked 或超 max_rounds。步数未知时用；路径已知用 enqueue_agent_turns_for_paths。回执=已入队未落盘。"),
		mcp.WithString("objective", mcp.Required(), mcp.Description("目标陈述（正典；勿缩水）")),
		mcp.WithString("driver", mcp.Description("驱动，空=默认")),
		mcp.WithString("title", mcp.Description("列表标题")),
		mcp.WithNumber("max_rounds", mcp.Description("外环硬上限，默认 100")),
		mcp.WithString("session", mcp.Description("空/active=侧栏当前会话；isolated=隐藏 once:queue")),
	), h.toolEnqueueGoal)

	s.AddTool(mcp.NewTool("enqueue_agent_turns_for_paths",
		mcp.WithDescription("按路径入队批任务（回执=已入队未落盘）。内部串行多节；prompt_template 可含 {path}。最多 50 路径。"),
		mcp.WithArray("rel_paths", mcp.Required(), mcp.Description("相对路径列表"), mcp.WithStringItems()),
		mcp.WithString("prompt_template", mcp.Description("用户任务模板；可含 {path} 占位（不自动追加路径）")),
		mcp.WithString("driver", mcp.Description("驱动，空=默认")),
	), h.toolEnqueueAgentTurnsForPaths)

	s.AddTool(mcp.NewTool("list_queue",
		mcp.WithDescription("队列结构化快照（status/steps/进度；无 prompt 全文）。入队≠落盘；done 仍须核实正文。"),
	), h.toolListQueue)

	s.AddTool(mcp.NewTool("cancel_queue_task",
		mcp.WithDescription("取消排队中或打断执行中的任务。"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("任务 id")),
	), h.toolCancelQueueTask)

	s.AddTool(mcp.NewTool("steer_queue_task",
		mcp.WithDescription("运行中插话（steer）：不取消当前任务；下一工具结果或下一 Goal 轮注入 [user_steering]。task_id 空=当前 running。"),
		mcp.WithString("text", mcp.Required(), mcp.Description("插话正文")),
		mcp.WithString("task_id", mcp.Description("任务 id；空=当前 running")),
	), h.toolSteerQueueTask)

	s.AddTool(mcp.NewTool("set_queue_paused",
		mcp.WithDescription("暂停/继续队列调度（不取消当前 running，除非再 cancel）。"),
		mcp.WithBoolean("paused", mcp.Description("true=暂停，默认 true")),
	), h.toolSetQueuePaused)
}

func NewMCPServer(h *Gateway, name, version, instructions string) *server.MCPServer {
	opts := []server.ServerOption{server.WithToolCapabilities(true)}
	if instructions != "" {
		opts = append(opts, server.WithInstructions(instructions))
	}
	s := server.NewMCPServer(name, version, opts...)
	RegisterCoreTools(s, h)
	return s
}

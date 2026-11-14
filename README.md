# ningharness

**English** | [中文](README.zh-CN.md)

**Pure Go backend framework** for hosting agents (SQLite + Gateway/MCP).  
No Wails, Node, or git required—only Go 1.25+.

Owns world truth and the tool gateway—not how the model thinks.

## Positioning

| In scope | Out of scope |
|----------|--------------|
| **Store** durable state (`desk.db`, …) | **Client** UI / product shells |
| Workspace I/O + **Gateway** MCP core tools | Client-specific tools and UX |
| Skill contract + Lesson / Memory | Client Skill catalogs and policies |
| Job / Task (part of Store) | — |
| **Lifecycle** (default steps + event bus) | — |
| **Optional defaults**: MCP server + Eino Guest | You may replace or disable them |

Tool truth lives in the host: Guests change the world only through Gateway—no bypass disk writes.  
Each Chat / Job run follows **`lifecycle`**. Phase boundaries use the **Bus** (`On` / `Watch`) with `RunState`. MCP tools run only inside `run_guest` via Gateway.

## Standard shape

```text
[Client] caller (app / MCP host / examples embedding this library)
        │  Open · UseProject · RunTurn/Chat · register tools · Bus
        ▼
[ningharness core]
   Workspace · Store · Gateway · Lifecycle · RunState
   swappable slots: Guest · External · Memory · Skill
```

- **Client**: the caller; UI is not implemented in this library.  
- **Store**: in-framework durable state (session / history / task / job / resource / lesson, …).  
- **Feedforward**: a `RunState` field written at `assemble` and sent into the model—not the whole `RunState`.

## Glossary

| Term | Meaning |
|------|---------|
| **Client** | Caller that embeds or invokes this library |
| **Harness** | Facade: Store + Session + Job |
| **Store** | Structured runtime state only (session / history / task / job / resource, …); experience bodies are Markdown |
| **Workspace** | Project file world (paths relative to root) |
| **Lifecycle** | Fixed-phase pipeline for one Task / Chat turn (steps + Bus) |
| **Bus / Event** | Lifecycle events: `Phase × before\|after`; `On` may Block, `Watch` observes only |
| **RunState** | Per-turn pipeline context; shared by steps and Bus; **Guest does not read it directly** |
| **Feedforward** | Extra model context on `RunState`; attached when `assemble` persists the user row |
| **Step** | Domain atom (e.g. `begin_task`, `run_guest`); **not** a tool name |
| **Gateway** | Tool gateway + MCP core (register / authorize / invoke); sole side-effect entry |
| **Guest** | Local model-loop slot: `Run(Input{Message,Feedforward})`; default Eino |
| **External** | Out-of-process brain (Cursor/Pi/dsh…): `RunTurn` + events; tools via host `guest.Tools` → Gateway |
| **Memory** | Recall slot: `assemble` reads Lesson MD into feedforward; default `memory.Lesson` |
| **Learning** | `Evidence → Candidate(JSONL) → Policy → Commit(MD)`; Lifecycle `learn`; default NoopPolicy |
| **Skill** | Method-pack slot: `List` / `Match` / `Load`; default `skill.Disk`; Learn does not patch `SKILL.md` |
| **Lesson** | Experience **Markdown SSOT**: project=`MEMORY.md`, personal=`USER.md`, skill=`LESSONS.md` |
| **Task** | One execution record (Store) |
| **Job** | Queue unit (schedules when to run a Lifecycle; may span tasks) |
| **Goal** | Outer-loop Job: re-trigger Lifecycle until `GOAL.yaml` status is terminal |
| **Spawn** | Subagent: fresh Session + `SpawnPacket`; registry `SPAWN.yaml`; parent manages via spawn_agent/list/steer/stop; no parent transcript by default |
| **Trace** | Append-only JSONL under `.ningharness/traces/` per Task; resume = paired tool_call/tool_result + task_end |

## Packages (by layer)

```text
ningharness/           facade Open/Close/UseProject
  lifecycle/           one turn: steps + Bus + RunState + Runner
  toolgateway/         Gateway + MCP + tool Registry; turn* = RunState projection
  workspace/ protocol/ files / shared DTOs
  store/ session/ history/ resource/   Store
  task/ job/ goal/ trace/
  skill/ lesson/       Skill Slot + Lesson (MD SSOT)
  memory/ learning/    Recall + Learning loop
  guest/ (+ eino/)     Guest slot (Run)
  defaults/            wiring (Lifecycle Host + Gateway + MCP + Eino + Memory + Learner + Skill)
  examples/            sample Client
```

Dependency rule: `defaults` → lifecycle / toolgateway / guest / memory / learning / skill; `lifecycle` does not import `toolgateway` (Host injected).

Storage: **DB = structured runtime**; narrative experience/skills = **Markdown**; candidates = JSONL (not DB).

Turn projection: `begin_task` → `Gateway.ProjectTurn`; teardown only via `OnExit` → `FinishTurn` (`end_task` is a Bus hook).  
`runLifecycle` may wrap ctx with `WithRunState` for Host steps; **Guest stays free of RunState** — tool turn identity is the Gateway projection.  
`assemble_context`: match `skill.paths` → merge caller feedforward + **Memory.Assemble** (MD) → `RunState.Feedforward` → persist user.  
`run_guest`: `Guest.Run` (`guest.Wire` merges feedforward into the model turn).  
`persist_turn`: persist assistant + drain tool errors.  
`learn`: Evidence → Candidate → Policy → optional MD Commit (default Noop).  
Tool dispatch: `RegisterHandler` + core `ensureCoreHandlers`; Client extensions need not edit `CallNamedTool`.

## Quick start

One complete path: edit Eino config → start MCP + Guest → send one message → paste Cursor URL.

Edit [`examples/chat/main.go`](examples/chat/main.go):

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/wcoreing/ningharness"
	"github.com/wcoreing/ningharness/defaults"
	"github.com/wcoreing/ningharness/guest/eino"
)

var einoCfg = eino.Opts{
	APIKey:  "sk-...",                    // required for Guest
	BaseURL: "https://api.openai.com/v1", // gateway goes here
	Model:   "gpt-4o-mini",
}

func main() {
	root, msg := ".", "List the project files in one short paragraph."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	if len(os.Args) > 2 {
		msg = os.Args[2]
	}
	abs, _ := filepath.Abs(root)

	rt, err := defaults.Open(defaults.Opts{
		Opts: ningharness.Opts{DataDir: filepath.Join(abs, ".ningharness-data"), Root: abs},
		Eino: einoCfg,
		// WithoutEino: true  → MCP only (no key)
		// MCPAddr: "off"     → Chat only (no HTTP)
		// MCPAddr: "127.0.0.1:51021" → fixed port
	})
	if err != nil {
		panic(err)
	}
	defer rt.Close()

	fmt.Println("MCP:", rt.MCPURL())
	// paste into ~/.cursor/mcp.json → {"mcpServers":{"ningharness":{"url":"<MCP URL>"}}}

	reply, err := rt.Chat(context.Background(), msg)
	if err != nil {
		panic(err)
	}
	fmt.Println(reply)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
}
```

Run:

```bash
go run ./examples/chat /path/to/project "List files briefly."
```

Defaults wire **Gateway** core tools + **MCP HTTP** (`/mcp`) + **Eino Guest** (ReAct via `Gateway.Invoke`).  
`WithoutEino` / `MCPAddr: "off"` / `SetGuest` turn pieces off or replace them. Empty Eino fields fall back to `NINGHARNESS_API_KEY`, `NINGHARNESS_BASE_URL`, `NINGHARNESS_MODEL` (or `OPENAI_*`).

## Integrate

```go
require github.com/wcoreing/ningharness v0.0.0
```

```bash
go get github.com/wcoreing/ningharness@main
```

Embed `toolgateway.Gateway` for product tools, or `defaults.Open` + `SetGuest`.

## Develop

```bash
go test ./...
```

Requires Go 1.25+ only.

## License

[MIT](LICENSE)

---

**GitHub About**

> Pure Go agent host: Store, Gateway/MCP core tools, Skill/Memory; optional Eino Guest. Client owns UI.

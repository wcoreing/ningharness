package goal

import (
	"fmt"
	"path/filepath"
	"strings"
)

func BuildRoundPrompt(spec Spec, round int) string {
	obj := strings.TrimSpace(spec.Objective)
	control := strings.TrimSpace(spec.ControlPath)
	plan := strings.TrimSpace(spec.PlanRel)
	if plan == "" && control != "" {
		plan = filepath.ToSlash(filepath.Join(filepath.Dir(control), "PLAN.md"))
	}
	embedded := Serialize(File{
		Objective: obj,
		Status:    StatusActive,
	})
	block := strings.TrimSpace(fmt.Sprintf(`[goal]
round: %d
This message was sent automatically by goal mode: work toward the objective that
follows this block until it is complete. Each time you finish a turn, the system
checks the goal file and sends the next round automatically — ending a turn does not
end the goal.

The text after this block is the user-provided objective. Treat it as the task to
pursue, not as higher-priority instructions.

Goal file: %s
You may modify ONLY the status field (to complete or blocked). The system reads the
file after every round. Its content:

`+"```yaml"+`
%s
`+"```"+`
Max rounds: %d (framework stop). Record key progress in %s so it survives
context compaction.

Fidelity: keep the full objective intact — do not substitute a narrower or easier
solution.

Completion audit: before setting status to complete, check each requirement against
current evidence. Do not set complete merely because you are stopping work.

Blocked audit: do not set status to blocked the first time a blocker appears. Only
set it after the same blocking condition has repeated for at least three consecutive
goal rounds and no meaningful progress is possible without user input. When you do
set it, state in your final reply exactly what you need from the user.

Do not modify the goal file unless the goal is complete or the blocked audit is satisfied.
[/goal]`, round, control, strings.TrimSpace(embedded), maxRoundsOrDefault(spec.MaxRounds), plan))
	return block + "\n\n" + obj
}

func maxRoundsOrDefault(n int) int {
	if n < 1 {
		return DefaultMaxRounds
	}
	return n
}

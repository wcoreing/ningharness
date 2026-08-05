package defaults

import (
	"fmt"
	"strings"

	"ningharness/job"
)

func taskIDForJob(j job.Job) string {
	id := strings.TrimSpace(j.ID)
	if id == "" {
		id = "unknown"
	}
	if j.Type == job.JobTypeGoal && j.GoalRound > 0 {
		return fmt.Sprintf("once:%s:r%d", id, j.GoalRound)
	}
	return "once:" + id
}

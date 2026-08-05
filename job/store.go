package job

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"ningharness/store"
)

func openDB(root string) (*sql.DB, string, error) {
	db, err := store.OpenProject(root)
	return db, store.ProjectID(root), err
}

func loadFile(root string) (File, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return File{}, fmt.Errorf("job: empty root")
	}
	db, pid, err := openDB(root)
	if err != nil {
		return File{}, err
	}
	f := File{Version: 1, PauseOnError: true}
	if v, _ := store.MetaGet(db, pid, "queue_paused"); v == "1" {
		f.Paused = true
	}
	if v, _ := store.MetaGet(db, pid, "queue_pause_on_error"); v == "0" {
		f.PauseOnError = false
	} else {
		f.PauseOnError = true
	}
	f.PauseReason, _ = store.MetaGet(db, pid, "queue_pause_reason")
	if v, _ := store.MetaGet(db, pid, "queue_max_parallel"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.MaxParallel = n
		}
	}

	rows, err := db.Query(`SELECT id, type, title, prompt, driver, model, target_rel, status, task_id, error, last_error,
		retry_count, batch_id, created_at, started_at, finished_at, session_key, purpose, feed_extra,
		step_done, step_total, progress_hint, goal_max_rounds, goal_round, steer_pending, sort_ord FROM jobs WHERE project_id=? ORDER BY sort_ord ASC, created_at ASC`, pid)
	if err != nil {
		return File{}, err
	}
	type head struct {
		j   Job
		ord int
	}
	var heads []head
	for rows.Next() {
		var h head
		if err := rows.Scan(&h.j.ID, &h.j.Type, &h.j.Title, &h.j.Prompt, &h.j.Driver, &h.j.Model, &h.j.TargetRel, &h.j.Status, &h.j.TaskID, &h.j.Error, &h.j.LastError,
			&h.j.RetryCount, &h.j.BatchID, &h.j.CreatedAt, &h.j.StartedAt, &h.j.FinishedAt, &h.j.SessionKey, &h.j.Purpose, &h.j.FeedExtra,
			&h.j.StepDone, &h.j.StepTotal, &h.j.ProgressHint, &h.j.GoalMaxRounds, &h.j.GoalRound, &h.j.SteerPending, &h.ord); err != nil {
			rows.Close()
			return File{}, err
		}
		heads = append(heads, h)
	}
	if err := rows.Close(); err != nil {
		return File{}, err
	}
	if err := rows.Err(); err != nil {
		return File{}, err
	}
	for _, h := range heads {
		job := h.j
		steps, err := loadSteps(db, pid, job.ID)
		if err != nil {
			return File{}, err
		}
		job.Steps = steps
		if job.Status == StatusRunning {
			job.Status = StatusError
			if job.Error == "" {
				job.Error = "进程中断（启动时残留 running）"
			}
		}
		f.Jobs = append(f.Jobs, job)
	}
	if len(f.Jobs) == 0 {
		if restored := restoreJobsFromTasks(root); len(restored) > 0 {
			f.Jobs = restored
			_ = saveFile(root, f)
		}
	}
	return f, nil
}

func loadSteps(db *sql.DB, pid, jobID string) ([]Step, error) {
	rows, err := db.Query(`SELECT rel, title, prompt, status, task_id, error FROM job_steps WHERE project_id=? AND job_id=? ORDER BY idx ASC`, pid, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.Rel, &s.Title, &s.Prompt, &s.Status, &s.TaskID, &s.Error); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func saveFile(root string, f File) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("job: empty root")
	}
	db, pid, err := openDB(root)
	if err != nil {
		return err
	}
	f.Version = 1
	f.History = nil
	f.LegacyTasks = nil
	f.Jobs = trimFinished(f.Jobs, maxFinishedKeep)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	metas := map[string]string{
		"queue_paused":         boolStr(f.Paused),
		"queue_pause_on_error": boolStr(f.PauseOnError),
		"queue_pause_reason":   f.PauseReason,
		"queue_max_parallel":   strconv.Itoa(f.MaxParallel),
	}
	for k, v := range metas {
		if _, err := tx.Exec(`INSERT INTO meta(project_id, key, value) VALUES(?, ?, ?)
			ON CONFLICT(project_id, key) DO UPDATE SET value=excluded.value`, pid, k, v); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM job_steps WHERE project_id=?`, pid); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM jobs WHERE project_id=?`, pid); err != nil {
		return err
	}
	for i, job := range f.Jobs {
		if _, err := tx.Exec(`INSERT INTO jobs(
			project_id, id, type, title, prompt, driver, model, target_rel, status, task_id, error, last_error,
			retry_count, batch_id, created_at, started_at, finished_at, session_key, purpose, feed_extra,
			step_done, step_total, progress_hint, goal_max_rounds, goal_round, steer_pending, sort_ord)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			pid, job.ID, job.Type, job.Title, job.Prompt, job.Driver, job.Model, job.TargetRel, string(job.Status), job.TaskID, job.Error, job.LastError,
			job.RetryCount, job.BatchID, job.CreatedAt, job.StartedAt, job.FinishedAt, job.SessionKey, job.Purpose, job.FeedExtra,
			job.StepDone, job.StepTotal, job.ProgressHint, job.GoalMaxRounds, job.GoalRound, job.SteerPending, i); err != nil {
			return err
		}
		for j, s := range job.Steps {
			if _, err := tx.Exec(`INSERT INTO job_steps(project_id, job_id, idx, rel, title, prompt, status, task_id, error) VALUES(?,?,?,?,?,?,?,?,?)`,
				pid, job.ID, j, s.Rel, s.Title, s.Prompt, string(s.Status), s.TaskID, s.Error); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func mergeHistoryIntoJobs(jobs, history []Job) []Job {
	seen := make(map[string]struct{}, len(jobs)+len(history))
	out := make([]Job, 0, len(jobs)+len(history))
	for _, t := range jobs {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, t)
	}
	for _, t := range history {
		id := strings.TrimSpace(t.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, t)
	}
	return out
}

func trimFinished(jobs []Job, keep int) []Job {
	if keep <= 0 || len(jobs) <= keep*2 {
		return jobs
	}
	var active, finished []Job
	for _, t := range jobs {
		switch t.Status {
		case StatusQueued, StatusRunning:
			active = append(active, t)
		default:
			finished = append(finished, t)
		}
	}
	if len(finished) <= keep {
		return jobs
	}
	finished = finished[len(finished)-keep:]
	out := make([]Job, 0, len(active)+len(finished))
	out = append(out, finished...)
	out = append(out, active...)
	return out
}

func computeStats(jobs []Job) Stats {
	var s Stats
	for _, t := range jobs {
		switch t.Status {
		case StatusQueued:
			s.Queued++
		case StatusRunning:
			s.Running++
		case StatusDone:
			s.Done++
		case StatusError:
			s.Error++
		case StatusCancelled:
			s.Cancelled++
		}
	}
	return s
}

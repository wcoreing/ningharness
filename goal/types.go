package goal

const DefaultMaxRounds = 100

type Status string

const (
	StatusActive   Status = "active"
	StatusComplete Status = "complete"
	StatusBlocked  Status = "blocked"
)

type Outcome string

const (
	OutcomeComplete  Outcome = "complete"
	OutcomeBlocked   Outcome = "blocked"
	OutcomeMaxRounds Outcome = "max_rounds"
	OutcomeAborted   Outcome = "aborted"
)

type Spec struct {
	Objective   string
	ControlPath string
	PlanRel     string
	MaxRounds   int
}

type File struct {
	Objective string `yaml:"objective"`
	Status    Status `yaml:"status"`
}

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

// File GOAL.yaml 控制面。
type File struct {
	Objective string `yaml:"objective"`
	Status    Status `yaml:"status"`
	// Reason status=blocked 时的受阻说明（给人/resume/lesson）；其它状态可空。
	Reason string `yaml:"reason,omitempty"`
	// Next 遗留字段；读入保留、不强制写入。
	Next string `yaml:"next,omitempty"`
}

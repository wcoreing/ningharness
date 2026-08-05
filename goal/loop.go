package goal

import (
	"context"
	"fmt"
	"strings"
)

type RoundExec func(ctx context.Context, wirePrompt string, round int) error

type ProgressFunc func(round int)

func Run(ctx context.Context, spec Spec, exec RoundExec, onProgress ProgressFunc) (Outcome, error) {
	if exec == nil {
		return OutcomeAborted, fmt.Errorf("goal: nil RoundExec")
	}
	obj := strings.TrimSpace(spec.Objective)
	if obj == "" {
		return OutcomeAborted, fmt.Errorf("goal: empty objective")
	}
	control := strings.TrimSpace(spec.ControlPath)
	if control == "" {
		return OutcomeAborted, fmt.Errorf("goal: empty control path")
	}
	maxRounds := maxRoundsOrDefault(spec.MaxRounds)
	if err := WriteOnce(control, obj); err != nil {
		return OutcomeAborted, err
	}
	spec.Objective = obj
	spec.ControlPath = control
	spec.MaxRounds = maxRounds

	for round := 1; round <= maxRounds; round++ {
		if ctx.Err() != nil {
			return OutcomeAborted, ctx.Err()
		}
		if onProgress != nil {
			onProgress(round)
		}
		wire := BuildRoundPrompt(spec, round)
		if err := exec(ctx, wire, round); err != nil {
			if ctx.Err() != nil {
				return OutcomeAborted, ctx.Err()
			}
			return OutcomeAborted, err
		}
		if ctx.Err() != nil {
			return OutcomeAborted, ctx.Err()
		}
		st := ReadStatus(control)
		switch st {
		case StatusComplete:
			return OutcomeComplete, nil
		case StatusBlocked:
			return OutcomeBlocked, nil
		case StatusActive:
			continue
		default:
			return OutcomeBlocked, nil
		}
	}
	return OutcomeMaxRounds, fmt.Errorf("goal: max rounds %d", maxRounds)
}

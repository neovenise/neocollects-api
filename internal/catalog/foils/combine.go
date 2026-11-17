package foils

import (
	"context"
	"errors"
)

type Runner interface {
	Run(ctx context.Context) (Result, error)
}

// All runs foil importers in turn and sums their results.
func All(runners ...Runner) Runner { return runnerList(runners) }

type runnerList []Runner

func (l runnerList) Run(ctx context.Context) (Result, error) {
	var total Result
	var errs []error
	for _, runner := range l {
		res, err := runner.Run(ctx)
		total.Sets += res.Sets
		total.Foils += res.Foils
		total.Unmatched += res.Unmatched
		total.FailedSets = append(total.FailedSets, res.FailedSets...)
		if err != nil {
			errs = append(errs, err)
			if ctx.Err() != nil {
				break
			}
		}
	}
	return total, errors.Join(errs...)
}

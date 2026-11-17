package mirror

import "context"

type Runner interface {
	Run(ctx context.Context, concurrency int) (Result, error)
}

// All runs copiers in turn and sums their results.
func All(runners ...Runner) Runner { return runnerList(runners) }

type runnerList []Runner

func (l runnerList) Run(ctx context.Context, concurrency int) (Result, error) {
	var total Result
	for _, runner := range l {
		res, err := runner.Run(ctx, concurrency)
		total.Copied += res.Copied
		total.UpToDate += res.UpToDate
		total.Failed = append(total.Failed, res.Failed...)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

package run

import (
	"booster/internal/model"
	"context"
	"time"
)

func Apply(ctx context.Context, plan PlanResult, emit func(model.Event)) model.Summary {
	started := time.Now()
	summary := model.Summary{Total: len(plan.Steps)}
	emit(model.RunStart{Total: len(plan.Steps)})

	failed := false
	for i, p := range plan.Steps {
		var result model.Result
		if failed {
			result = model.Result{Status: model.StatusSkipped, Change: p.Change, Reason: "not run"}
		} else {
			emit(model.TaskStart{Index: i, Name: p.Name, Change: p.Change})
			result = applyStep(ctx, p)
			failed = result.Status == model.StatusFailed
		}
		count(&summary, result.Status)
		emit(model.TaskDone{Index: i, Result: result})
	}

	summary.Duration = time.Since(started)
	emit(model.RunDone{Summary: summary})
	return summary
}

func applyStep(ctx context.Context, p Planned) model.Result {
	if p.Change.Kind == model.ChangeNone {
		return model.Result{Status: model.StatusOK, Change: p.Change}
	}

	started := time.Now()
	err := p.Action.Apply(ctx)
	result := model.Result{Status: model.StatusChanged, Change: p.Change, Err: err, Duration: time.Since(started)}
	if err != nil {
		result.Status = model.StatusFailed
	}
	return result
}

func count(s *model.Summary, status model.Status) {
	switch status {
	case model.StatusOK:
		s.OK++
	case model.StatusChanged:
		s.Changed++
	case model.StatusSkipped:
		s.Skipped++
	case model.StatusUnknown:
		s.Unknown++
	case model.StatusFailed:
		s.Failed++
	}
}

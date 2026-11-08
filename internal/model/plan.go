package model

type PlannedStep struct {
	Name   string
	Change Change
}

type Plan struct {
	Steps    []PlannedStep
	Excluded int
}

func (p Plan) HasChanges() bool {
	for _, s := range p.Steps {
		if s.Change.Kind != ChangeNone {
			return true
		}
	}
	return false
}

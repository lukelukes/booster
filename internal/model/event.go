package model

type Event interface{ isEvent() }

type RunStart struct {
	Config  string
	OS      string
	Profile string
	Total   int
}

type TaskStart struct {
	Name   string
	Change Change
	Index  int
}

type TaskOutput struct {
	Line  string
	Index int
}

type TaskDone struct {
	Result Result
	Index  int
}

type RunDone struct {
	Summary Summary
}

func (RunStart) isEvent()   {}
func (TaskStart) isEvent()  {}
func (TaskOutput) isEvent() {}
func (TaskDone) isEvent()   {}
func (RunDone) isEvent()    {}

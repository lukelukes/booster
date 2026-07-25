package model

var (
	_ Event = RunStart{}
	_ Event = TaskStart{}
	_ Event = TaskOutput{}
	_ Event = TaskDone{}
	_ Event = RunDone{}
)

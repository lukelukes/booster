package model

import "time"

type ChangeKind int

const (
	ChangeNone ChangeKind = iota
	ChangeCreate
	ChangeUpdate
	ChangeUnknown
)

type Item struct {
	Name string
	From string
	To   string
}

type Change struct {
	Verb      string
	Items     []Item
	Kind      ChangeKind
	Total     int
	NeedsSudo bool
}

type Status int

const (
	StatusOK Status = iota
	StatusChanged
	StatusSkipped
	StatusUnknown
	StatusFailed
)

type Result struct {
	Err      error
	Reason   string
	Change   Change
	Status   Status
	Duration time.Duration
}

type Summary struct {
	Total    int
	OK       int
	Changed  int
	Skipped  int
	Unknown  int
	Failed   int
	Duration time.Duration
}

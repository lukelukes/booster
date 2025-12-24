package coordinator

import (
	"booster/internal/task"
)

type TaskCompleteMsg struct {
	TaskIndex int
	Result    task.Result
	Logs      []string
}

type Coordinator struct {
	logHistory  map[int][]string
	currentLogs []string
	currentTask int

	pendingResult *task.Result
	logsDone      bool
}

func New() *Coordinator {
	return &Coordinator{
		logHistory: make(map[int][]string),
	}
}

func (c *Coordinator) StartTask(taskIndex int) {
	c.currentTask = taskIndex
	c.currentLogs = nil
	c.pendingResult = nil
	c.logsDone = false
}

func (c *Coordinator) AddLogLine(line string) {
	c.currentLogs = append(c.currentLogs, line)
}

func (c *Coordinator) CurrentLogs() []string {
	return c.currentLogs
}

func (c *Coordinator) LogsFor(taskIndex int) []string {
	return c.logHistory[taskIndex]
}

func (c *Coordinator) LogsDone() *TaskCompleteMsg {
	c.logsDone = true

	if c.pendingResult != nil {
		return c.complete(*c.pendingResult)
	}
	return nil
}

func (c *Coordinator) TaskDone(result task.Result) *TaskCompleteMsg {
	if c.pendingResult == nil && !c.logsDone {
		c.pendingResult = &result
		return nil
	}

	if !c.logsDone {
		return nil
	}

	return c.complete(result)
}

func (c *Coordinator) complete(result task.Result) *TaskCompleteMsg {
	if len(c.currentLogs) > 0 {
		c.logHistory[c.currentTask] = c.currentLogs
	}

	msg := &TaskCompleteMsg{
		TaskIndex: c.currentTask,
		Result:    result,
		Logs:      c.currentLogs,
	}

	c.currentLogs = nil
	c.pendingResult = nil
	c.logsDone = false

	return msg
}

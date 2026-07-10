package task

import "context"

type ExecuteParams struct {
	TaskID          string
	TaskType        string
	BotID           string
	SessionKey      string
	Route           string
	Content         string
	GlobalTaskName  string
}

type ExecuteFunc func(ctx context.Context, params ExecuteParams) error

type Executor struct {
	executeFn ExecuteFunc
}

func NewExecutor(fn ExecuteFunc) *Executor {
	return &Executor{executeFn: fn}
}

func (e *Executor) Execute(ctx context.Context, params ExecuteParams) error {
	return e.executeFn(ctx, params)
}

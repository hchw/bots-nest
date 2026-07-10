package task

import (
	"context"
	"fmt"
	"log"
	"sync"
)

type Engine struct {
	store    *Store
	plugin   Plugin
	executor *Executor
	mu       sync.RWMutex
	jobMap   map[string]JobID
}

func NewEngine(store *Store, plugin Plugin) *Engine {
	return &Engine{
		store:  store,
		plugin: plugin,
		jobMap: make(map[string]JobID),
	}
}

func (e *Engine) SetExecutor(executor *Executor) {
	e.executor = executor
}

func (e *Engine) Start() error {
	if e.executor == nil {
		return fmt.Errorf("executor 未设置，无法启动引擎")
	}

	if err := e.plugin.Init(nil); err != nil {
		return fmt.Errorf("初始化插件失败: %w", err)
	}

	sessionTasks, err := e.store.ListAllEnabledSessionTasks()
	if err != nil {
		return fmt.Errorf("查询启用 session 任务失败: %w", err)
	}
	for _, t := range sessionTasks {
		taskDef := &TaskDefinition{
			ID:          t.ID,
			TaskType:    t.TaskType,
			CronExpr:    t.CronExpr,
			IntervalSec: t.IntervalSec,
		}
		if t.RunAt != nil {
			taskDef.RunAt = t.RunAt.Format("2006-01-02T15:04:05Z07:00")
		}
		execFn := e.makeTaskExecFn(t.ID, "session", t.BotID, t.SessionKey, t.Route, t.Content)
		jobID, err := e.plugin.Schedule(taskDef, execFn)
		if err != nil {
			log.Printf("[引擎] 恢复 session 任务 %s 失败: %v", t.ID, err)
		} else {
			e.mu.Lock()
			e.jobMap[t.ID] = jobID
			e.mu.Unlock()
			log.Printf("[引擎] 恢复 session 任务 %s", t.ID)
		}
	}

	globalTasks, err := e.store.ListAllEnabledGlobalTasks()
	if err != nil {
		return fmt.Errorf("查询启用全局任务失败: %w", err)
	}
	for _, t := range globalTasks {
		bindings, err := e.store.ListBindingsByTask(t.ID)
		if err != nil {
			log.Printf("[引擎] 查询全局任务 %s 绑定关系失败: %v", t.ID, err)
			continue
		}
		taskDef := &TaskDefinition{
			ID:          t.ID,
			TaskType:    t.TaskType,
			CronExpr:    t.CronExpr,
			IntervalSec: t.IntervalSec,
		}
		execFn := e.makeGlobalTaskExecFn(t, bindings)
		jobID, err := e.plugin.Schedule(taskDef, execFn)
		if err != nil {
			log.Printf("[引擎] 恢复全局任务 %s 失败: %v", t.ID, err)
		} else {
			e.mu.Lock()
			e.jobMap[t.ID] = jobID
			e.mu.Unlock()
			log.Printf("[引擎] 恢复全局任务 %s (%s)", t.ID, t.Name)
		}
	}

	log.Printf("[引擎] 启动完成, 恢复 %d session 任务, %d 全局任务", len(sessionTasks), len(globalTasks))
	return nil
}

func (e *Engine) makeTaskExecFn(taskID, taskType, botID, sessionKey, route, content string) func() {
	return func() {
		params := ExecuteParams{
			TaskID:     taskID,
			TaskType:   taskType,
			BotID:      botID,
			SessionKey: sessionKey,
			Route:      route,
			Content:    content,
		}
		if err := e.executor.Execute(context.Background(), params); err != nil {
			log.Printf("[引擎] 执行任务 %s 失败: %v", taskID, err)
		}
	}
}

func (e *Engine) makeGlobalTaskExecFn(t GlobalTask, bindings []GlobalTaskBinding) func() {
	return func() {
		for _, b := range bindings {
			params := ExecuteParams{
				TaskID:         t.ID,
				TaskType:       "global",
				BotID:          b.BotID,
				Route:          t.Route,
				Content:        t.Content,
				GlobalTaskName: t.Name,
			}
			if err := e.executor.Execute(context.Background(), params); err != nil {
				log.Printf("[引擎] 执行全局任务 %s 绑定机器人 %s 失败: %v", t.ID, b.BotID, err)
			}
		}
	}
}

func (e *Engine) ScheduleTask(def *TaskDefinition, botID, sessionKey, route, content, taskType string) (JobID, error) {
	execFn := e.makeTaskExecFn(def.ID, taskType, botID, sessionKey, route, content)
	jobID, err := e.plugin.Schedule(def, execFn)
	if err != nil {
		return "", fmt.Errorf("调度任务失败: %w", err)
	}
	e.mu.Lock()
	e.jobMap[def.ID] = jobID
	e.mu.Unlock()
	return jobID, nil
}

func (e *Engine) ScheduleGlobalTask(def *TaskDefinition, t GlobalTask, bindings []GlobalTaskBinding) (JobID, error) {
	execFn := e.makeGlobalTaskExecFn(t, bindings)
	jobID, err := e.plugin.Schedule(def, execFn)
	if err != nil {
		return "", fmt.Errorf("调度全局任务失败: %w", err)
	}
	e.mu.Lock()
	e.jobMap[def.ID] = jobID
	e.mu.Unlock()
	return jobID, nil
}

func (e *Engine) CancelTask(taskID string) error {
	e.mu.RLock()
	jobID, ok := e.jobMap[taskID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("任务 %s 未找到调度记录", taskID)
	}
	if err := e.plugin.Cancel(jobID); err != nil {
		return fmt.Errorf("取消调度失败: %w", err)
	}
	e.mu.Lock()
	delete(e.jobMap, taskID)
	e.mu.Unlock()
	return nil
}

func (e *Engine) CancelJob(jobID JobID) error {
	return e.plugin.Cancel(jobID)
}

func (e *Engine) Shutdown() error {
	return e.plugin.Shutdown()
}

func (e *Engine) Store() *Store {
	return e.store
}

func (e *Engine) PluginType() string {
	return e.plugin.Type()
}

package task

import (
	"encoding/json"
	"fmt"
	"sync"
)

type JobID string

type TaskDefinition struct {
	ID          string
	TaskType    string
	CronExpr    string
	IntervalSec int
	RunAt       string
}

type Plugin interface {
	Type() string
	Name() string
	Init(config json.RawMessage) error
	Schedule(task *TaskDefinition, executeFn func()) (JobID, error)
	Cancel(jobID JobID) error
	Shutdown() error
}

type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

var globalRegistry = &Registry{
	plugins: make(map[string]Plugin),
}

func GlobalRegistry() *Registry {
	return globalRegistry
}

func (r *Registry) Register(p Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	t := p.Type()
	if _, exists := r.plugins[t]; exists {
		return fmt.Errorf("插件 %s 已注册", t)
	}
	r.plugins[t] = p
	return nil
}

func (r *Registry) GetByType(typeName string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[typeName]
	return p, ok
}

func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}

func (r *Registry) MustGet(typeName string) Plugin {
	p, ok := r.GetByType(typeName)
	if !ok {
		panic(fmt.Sprintf("插件 %s 未注册", typeName))
	}
	return p
}

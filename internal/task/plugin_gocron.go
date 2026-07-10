package task

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
)

const PluginTypeGocron = "gocron"

type GocronPlugin struct {
	name     string
	scheduler gocron.Scheduler
	jobMap   map[string]gocron.Job
}

func NewGocronPlugin() *GocronPlugin {
	return &GocronPlugin{
		name:   "gocron",
		jobMap: make(map[string]gocron.Job),
	}
}

func (p *GocronPlugin) Type() string {
	return PluginTypeGocron
}

func (p *GocronPlugin) Name() string {
	return p.name
}

func (p *GocronPlugin) Init(config json.RawMessage) error {
	s, err := gocron.NewScheduler()
	if err != nil {
		return fmt.Errorf("创建 gocron 调度器失败: %w", err)
	}
	p.scheduler = s
	s.Start()
	log.Println("[gocron] 调度器已启动")
	return nil
}

func (p *GocronPlugin) Schedule(task *TaskDefinition, executeFn func()) (JobID, error) {
	var job gocron.Job
	var err error

	switch task.TaskType {
	case "interval":
		if task.IntervalSec <= 0 {
			return "", fmt.Errorf("interval_sec 必须大于 0")
		}
		job, err = p.scheduler.NewJob(
			gocron.DurationJob(time.Duration(task.IntervalSec)*time.Second),
			gocron.NewTask(executeFn),
		)

	case "cron":
		if task.CronExpr == "" {
			return "", fmt.Errorf("cron_expr 不能为空")
		}
		job, err = p.scheduler.NewJob(
			gocron.CronJob(task.CronExpr, false),
			gocron.NewTask(executeFn),
		)

	case "once":
		runAt, parseErr := time.Parse(time.RFC3339, task.RunAt)
		if parseErr != nil {
			return "", fmt.Errorf("解析 run_at 失败: %w", parseErr)
		}
		job, err = p.scheduler.NewJob(
			gocron.OneTimeJob(gocron.OneTimeJobStartDateTime(runAt)),
			gocron.NewTask(executeFn),
		)

	default:
		return "", fmt.Errorf("不支持的 task_type: %s", task.TaskType)
	}

	if err != nil {
		return "", fmt.Errorf("创建调度任务失败: %w", err)
	}

	jobID := job.ID().String()
	p.jobMap[jobID] = job
	return JobID(jobID), nil
}

func (p *GocronPlugin) Cancel(jobID JobID) error {
	uid, err := uuid.Parse(string(jobID))
	if err != nil {
		return fmt.Errorf("解析 job ID 失败: %w", err)
	}
	job, exists := p.jobMap[string(jobID)]
	if !exists {
		return fmt.Errorf("job %s 未找到", jobID)
	}
	if err := p.scheduler.RemoveJob(uid); err != nil {
		return fmt.Errorf("移除 job 失败: %w", err)
	}
	delete(p.jobMap, string(jobID))
	_ = job
	return nil
}

func (p *GocronPlugin) Shutdown() error {
	if p.scheduler != nil {
		return p.scheduler.Shutdown()
	}
	return nil
}

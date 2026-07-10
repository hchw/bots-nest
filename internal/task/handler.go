package task

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ScheduledTaskHandler struct {
	store  *Store
	engine *Engine
}

func NewScheduledTaskHandler(store *Store, engine *Engine) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{store: store, engine: engine}
}

type STRequest struct {
	Operation   string `json:"operation"`
	TaskType    string `json:"task_type,omitempty"`
	CronExpr    string `json:"cron_expr,omitempty"`
	IntervalSec int    `json:"interval_sec,omitempty"`
	RunAt       string `json:"run_at,omitempty"`
	Content     string `json:"content,omitempty"`
	Route       string `json:"route,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
}

func (h *ScheduledTaskHandler) Handle(args json.RawMessage, botID, sessionKey string) string {
	var req STRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return "参数解析失败: " + err.Error()
	}

	switch req.Operation {
	case "create":
		return h.handleCreate(req, botID, sessionKey)
	case "cancel":
		return h.handleCancel(req, botID, sessionKey)
	case "list":
		return h.handleList(botID, sessionKey)
	default:
		return fmt.Sprintf("不支持的操作: %s", req.Operation)
	}
}

func (h *ScheduledTaskHandler) handleCreate(req STRequest, botID, sessionKey string) string {
	if req.Content == "" || req.Route == "" {
		return "参数错误: content 和 route 为必填项"
	}
	if req.TaskType == "" {
		return "参数错误: task_type 为必填项"
	}
	if req.Route != "llm" && req.Route != "direct" {
		return "参数错误: route 必须是 llm 或 direct"
	}

	taskID := uuid.New().String()
	now := time.Now()

	st := &SessionTask{
		ID:          taskID,
		BotID:       botID,
		SessionKey:  sessionKey,
		TaskType:    req.TaskType,
		CronExpr:    req.CronExpr,
		IntervalSec: req.IntervalSec,
		Route:       req.Route,
		Content:     req.Content,
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if req.TaskType == "once" {
		runAt, err := time.Parse(time.RFC3339, req.RunAt)
		if err != nil {
			runAt, err = time.Parse("2006-01-02T15:04:05", req.RunAt)
			if err != nil {
				return fmt.Sprintf("参数错误: run_at 格式无效 (%s)", err.Error())
			}
			runAt = runAt.In(time.Local)
		}
		st.RunAt = &runAt
	}

	if req.TaskType == "interval" && req.IntervalSec <= 0 {
		return "参数错误: interval_sec 必须大于 0"
	}
	if req.TaskType == "cron" && req.CronExpr == "" {
		return "参数错误: cron_expr 不能为空"
	}

	if err := h.store.CreateSessionTask(st); err != nil {
		return "创建任务失败: " + err.Error()
	}

	taskDef := &TaskDefinition{
		ID:          taskID,
		TaskType:    req.TaskType,
		CronExpr:    req.CronExpr,
		IntervalSec: req.IntervalSec,
	}
	if req.TaskType == "once" && st.RunAt != nil {
		taskDef.RunAt = st.RunAt.Format("2006-01-02T15:04:05Z07:00")
	}

	if _, err := h.engine.ScheduleTask(taskDef, botID, sessionKey, req.Route, req.Content, "session"); err != nil {
		st.Enabled = false
		h.store.UpdateSessionTask(st)
		return "调度任务失败: " + err.Error()
	}

	return fmt.Sprintf("✅ 定时任务已创建 (ID: %s)", taskID)
}

func (h *ScheduledTaskHandler) handleCancel(req STRequest, botID, sessionKey string) string {
	if req.TaskID == "" {
		return "参数错误: task_id 为必填项"
	}

	task, err := h.store.GetSessionTask(req.TaskID)
	if err != nil {
		return "未找到该任务"
	}
	if task.BotID != botID || task.SessionKey != sessionKey {
		return "无权操作该任务"
	}
	if !task.Enabled {
		return "该任务已被取消"
	}

	if err := h.engine.CancelTask(req.TaskID); err != nil {
		return "取消失败: " + err.Error()
	}

	task.Enabled = false
	task.UpdatedAt = time.Now()
	if err := h.store.UpdateSessionTask(task); err != nil {
		return "更新任务状态失败: " + err.Error()
	}

	return fmt.Sprintf("✅ 任务 %s 已取消", req.TaskID)
}

func (h *ScheduledTaskHandler) handleList(botID, sessionKey string) string {
	tasks, err := h.store.ListActiveSessionTasks(botID, sessionKey)
	if err != nil {
		return "查询任务失败: " + err.Error()
	}
	if len(tasks) == 0 {
		return "当前没有活跃的定时任务"
	}

	var result string
	for _, t := range tasks {
		timeInfo := ""
		switch t.TaskType {
		case "interval":
			timeInfo = fmt.Sprintf("每 %d 秒", t.IntervalSec)
		case "cron":
			timeInfo = t.CronExpr
		case "once":
			if t.RunAt != nil {
				timeInfo = t.RunAt.Format("01-02 15:04")
			}
		}
		routeLabel := map[string]string{"llm": "AI 处理", "direct": "直接推送"}[t.Route]
		result += fmt.Sprintf("- ID: %s | 类型: %s | 时间: %s | 路由: %s | 内容: %s\n",
			t.ID, t.TaskType, timeInfo, routeLabel, t.Content)
	}
	return result
}

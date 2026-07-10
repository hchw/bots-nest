package task

import "time"

type TaskPlugin struct {
	ID          string    `gorm:"primaryKey;size:255" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Type        string    `gorm:"not null" json:"type"`
	Description string    `gorm:"type:text" json:"description"`
	Enabled     bool      `gorm:"default:1" json:"enabled"`
	Config      string    `gorm:"type:text" json:"config"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type GlobalTask struct {
	ID           string    `gorm:"primaryKey;size:255" json:"id"`
	Name         string    `gorm:"not null" json:"name"`
	TaskType     string    `gorm:"not null" json:"task_type"`
	CronExpr     string    `gorm:"default:''" json:"cron_expr"`
	IntervalSec  int       `gorm:"default:0" json:"interval_sec"`
	Route        string    `gorm:"not null" json:"route"`
	Content      string    `gorm:"type:text;not null" json:"content"`
	Enabled      bool      `gorm:"default:1" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type GlobalTaskBinding struct {
	ID        string    `gorm:"primaryKey;size:255" json:"id"`
	TaskID    string    `gorm:"not null;index" json:"task_id"`
	BotID     string    `gorm:"not null;index" json:"bot_id"`
	CreatedAt time.Time `json:"created_at"`
}

type SessionTask struct {
	ID          string    `gorm:"primaryKey;size:255" json:"id"`
	BotID       string    `gorm:"not null;index" json:"bot_id"`
	SessionKey  string    `gorm:"not null;index" json:"session_key"`
	TaskType    string    `gorm:"not null" json:"task_type"`
	CronExpr    string    `gorm:"default:''" json:"cron_expr"`
	IntervalSec int       `gorm:"default:0" json:"interval_sec"`
	RunAt       *time.Time `json:"run_at"`
	Route       string    `gorm:"not null" json:"route"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	Enabled     bool      `gorm:"default:1" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskExecutionLog struct {
	ID          string    `gorm:"primaryKey;size:255" json:"id"`
	TaskID      string    `gorm:"not null;index" json:"task_id"`
	TaskType    string    `gorm:"not null" json:"task_type"`
	BotID       string    `gorm:"default:''" json:"bot_id"`
	SessionKey  string    `gorm:"default:''" json:"session_key"`
	Status      string    `gorm:"not null" json:"status"`
	Result      string    `gorm:"type:text" json:"result"`
	TriggerType string    `gorm:"not null" json:"trigger_type"`
	ExecutedAt  time.Time `json:"executed_at"`
}

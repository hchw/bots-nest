package task

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var TaskDB *gorm.DB

func InitDB(dsn string) {
	var err error
	TaskDB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("打开 tasks.db 失败: %v", err)
	}
	log.Println("tasks.db 连接成功")
}

func Migrate() {
	if err := TaskDB.AutoMigrate(
		&TaskPlugin{},
		&GlobalTask{},
		&GlobalTaskBinding{},
		&SessionTask{},
		&TaskExecutionLog{},
	); err != nil {
		log.Fatalf("tasks.db 迁移失败: %v", err)
	}
	log.Println("tasks.db 迁移完成")
}

type Store struct{}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) ListPlugins() ([]TaskPlugin, error) {
	var list []TaskPlugin
	err := TaskDB.Find(&list).Error
	return list, err
}

func (s *Store) GetPluginByType(typeName string) (*TaskPlugin, error) {
	var p TaskPlugin
	err := TaskDB.Where("type = ?", typeName).First(&p).Error
	return &p, err
}

func (s *Store) UpsertPlugin(p *TaskPlugin) error {
	return TaskDB.Save(p).Error
}

func (s *Store) ListGlobalTasks() ([]GlobalTask, error) {
	var list []GlobalTask
	err := TaskDB.Order("created_at DESC").Find(&list).Error
	return list, err
}

func (s *Store) GetGlobalTask(id string) (*GlobalTask, error) {
	var t GlobalTask
	err := TaskDB.Where("id = ?", id).First(&t).Error
	return &t, err
}

func (s *Store) CreateGlobalTask(t *GlobalTask) error {
	return TaskDB.Create(t).Error
}

func (s *Store) UpdateGlobalTask(t *GlobalTask) error {
	return TaskDB.Save(t).Error
}

func (s *Store) DeleteGlobalTask(id string) error {
	return TaskDB.Where("id = ?", id).Delete(&GlobalTask{}).Error
}

func (s *Store) ListBindingsByTask(taskID string) ([]GlobalTaskBinding, error) {
	var list []GlobalTaskBinding
	err := TaskDB.Where("task_id = ?", taskID).Find(&list).Error
	return list, err
}

func (s *Store) ListBindingsByBot(botID string) ([]GlobalTaskBinding, error) {
	var list []GlobalTaskBinding
	err := TaskDB.Where("bot_id = ?", botID).Find(&list).Error
	return list, err
}

func (s *Store) CreateBinding(b *GlobalTaskBinding) error {
	return TaskDB.Create(b).Error
}

func (s *Store) DeleteBindingsByTask(taskID string) error {
	return TaskDB.Where("task_id = ?", taskID).Delete(&GlobalTaskBinding{}).Error
}

func (s *Store) ListSessionTasks(botID, sessionKey string) ([]SessionTask, error) {
	var list []SessionTask
	err := TaskDB.Where("bot_id = ? AND session_key = ?", botID, sessionKey).Order("created_at DESC").Find(&list).Error
	return list, err
}

func (s *Store) ListActiveSessionTasks(botID, sessionKey string) ([]SessionTask, error) {
	var list []SessionTask
	err := TaskDB.Where("bot_id = ? AND session_key = ? AND enabled = 1", botID, sessionKey).Find(&list).Error
	return list, err
}

func (s *Store) GetSessionTask(id string) (*SessionTask, error) {
	var t SessionTask
	err := TaskDB.Where("id = ?", id).First(&t).Error
	return &t, err
}

func (s *Store) CreateSessionTask(t *SessionTask) error {
	return TaskDB.Create(t).Error
}

func (s *Store) UpdateSessionTask(t *SessionTask) error {
	return TaskDB.Save(t).Error
}

func (s *Store) ListAllEnabledSessionTasks() ([]SessionTask, error) {
	var list []SessionTask
	err := TaskDB.Where("enabled = 1").Find(&list).Error
	return list, err
}

func (s *Store) ListAllEnabledGlobalTasks() ([]GlobalTask, error) {
	var list []GlobalTask
	err := TaskDB.Where("enabled = 1").Find(&list).Error
	return list, err
}

func (s *Store) CreateExecutionLog(log *TaskExecutionLog) error {
	return TaskDB.Create(log).Error
}

func (s *Store) ListExecutionLogs(taskID string) ([]TaskExecutionLog, error) {
	var list []TaskExecutionLog
	err := TaskDB.Where("task_id = ?", taskID).Order("executed_at DESC").Find(&list).Error
	return list, err
}

func (s *Store) CreatePlugin(p *TaskPlugin) error {
	return TaskDB.Create(p).Error
}

func (s *Store) ListEnabledPlugins() ([]TaskPlugin, error) {
	var list []TaskPlugin
	err := TaskDB.Where("enabled = 1").Find(&list).Error
	return list, err
}

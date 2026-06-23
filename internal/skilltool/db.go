package skilltool

import (
	"github.com/hchw/bots-nest/internal/db"
)

func ListToolsBySkill(botID string, skillID uint) ([]db.GoJudgeTool, error) {
	var tools []db.GoJudgeTool
	err := db.DB.Where("bot_id = ? AND skill_id = ?", botID, skillID).Find(&tools).Error
	return tools, err
}

func GetToolByID(botID string, skillID uint, toolID uint) (*db.GoJudgeTool, error) {
	var tool db.GoJudgeTool
	err := db.DB.Where("id = ? AND bot_id = ? AND skill_id = ?", toolID, botID, skillID).First(&tool).Error
	if err != nil {
		return nil, err
	}
	return &tool, nil
}

func CreateTool(tool *db.GoJudgeTool) error {
	return db.DB.Create(tool).Error
}

func UpdateTool(botID string, skillID uint, toolID uint, updates map[string]interface{}) error {
	return db.DB.Model(&db.GoJudgeTool{}).
		Where("id = ? AND bot_id = ? AND skill_id = ?", toolID, botID, skillID).
		Updates(updates).Error
}

func DeleteTool(botID string, skillID uint, toolID uint) error {
	return db.DB.
		Where("id = ? AND bot_id = ? AND skill_id = ?", toolID, botID, skillID).
		Delete(&db.GoJudgeTool{}).Error
}

func ListToolsByBot(botID string) ([]db.GoJudgeTool, error) {
	var tools []db.GoJudgeTool
	err := db.DB.Where("bot_id = ?", botID).Find(&tools).Error
	return tools, err
}

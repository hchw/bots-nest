// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package db

import (
	"time"
)

type LLMProvider struct {
	ID        string    `gorm:"primaryKey;size:255" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Endpoint  string    `gorm:"not null" json:"endpoint"`
	APIKey    string    `gorm:"not null" json:"-"`
	Models    string    `gorm:"type:text;default:[]" json:"models"`
	Enabled   bool      `gorm:"default:1" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MCP struct {
	ID        string    `gorm:"primaryKey;size:255" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Type      string    `gorm:"default:url" json:"type"`
	Endpoint  string    `gorm:"default:''" json:"endpoint"`
	Command   string    `gorm:"default:''" json:"command"`
	Args      string    `gorm:"default:''" json:"args"`
	Env       string    `gorm:"default:''" json:"env"`
	Tools     string    `gorm:"type:text" json:"tools"`
	Enabled   bool      `gorm:"default:1" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Bot struct {
	ID               string    `gorm:"primaryKey;size:255" json:"id"`
	Name             string    `gorm:"not null" json:"name"`
	Status           string    `gorm:"default:disconnected" json:"status"`
	WecomBotID       string    `gorm:"not null" json:"-"`
	WecomSecret      string    `gorm:"not null" json:"-"`
	LLMProviderID    string    `gorm:"not null" json:"llm_provider_id"`
	LLMModel         string    `gorm:"default:gpt-4o" json:"llm_model"`
	LLMTemperature   *float64  `json:"llm_temperature"`
	LLMMaxTokens     *int      `json:"llm_max_tokens"`
	MaxSessionTokens int       `gorm:"default:4096" json:"max_session_tokens"`
	Enabled          bool      `gorm:"default:1" json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Skill struct {
	ID           uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	BotID        string `gorm:"not null;index" json:"bot_id"`
	Name         string `gorm:"not null" json:"name"`
	Description  string `gorm:"not null" json:"description"`
	SystemPrompt string `gorm:"type:text;not null" json:"system_prompt"`
	Tools        string `gorm:"type:text" json:"tools"`
	Enabled      bool   `gorm:"default:1" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	SessionKey       string    `gorm:"primaryKey;size:255" json:"session_key"`
	BotID            string    `gorm:"not null;index" json:"bot_id"`
	UserID           string    `gorm:"not null" json:"user_id"`
	UserName         string    `json:"user_name"`
	ConversationType string    `gorm:"not null" json:"conversation_type"`
	GroupID          string    `json:"group_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type GoJudgeTool struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BotID        string    `gorm:"not null;index" json:"bot_id"`
	SkillID      uint      `gorm:"not null;index" json:"skill_id"`
	Name         string    `gorm:"not null" json:"name"`
	Language     string    `gorm:"not null" json:"language"`
	Code         string    `gorm:"type:text" json:"code"`
	InputParams  string    `gorm:"type:text" json:"input_params"`
	OutputParams string    `gorm:"type:text" json:"output_params"`
	Prompt       string    `gorm:"type:text" json:"prompt"`
	Status       string    `gorm:"default:draft" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Message struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionKey string    `gorm:"not null;index" json:"session_key"`
	Role       string    `gorm:"not null" json:"role"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Tokens     int       `gorm:"default:0" json:"tokens"`
	Expired    bool      `gorm:"default:0" json:"expired"`
	CreatedAt  time.Time `json:"created_at"`
}

type KnowledgeBase struct {
	ID                  string    `gorm:"primaryKey;size:255" json:"id"`
	Name                string    `gorm:"not null" json:"name"`
	Description         string    `gorm:"type:text" json:"description"`
	AutoSummary         string    `gorm:"type:text" json:"auto_summary"`
	EmbeddingMode       string    `gorm:"size:50;default:provider" json:"embedding_mode"`
	EmbeddingProviderID string    `gorm:"size:255;not null" json:"embedding_provider_id"`
	EmbeddingModel      string    `gorm:"size:255;not null" json:"embedding_model"`
	FileCount           int       `gorm:"default:0" json:"file_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ImportTask struct {
	ID               string    `gorm:"primaryKey;size:255" json:"id"`
	KnowledgeBaseID  string    `gorm:"not null;index" json:"kb_id"`
	FileName         string    `gorm:"not null" json:"file_name"`
	FilePath         string    `gorm:"default:''" json:"file_path"`
	FileSize         int64     `json:"file_size"`
	Status           string    `gorm:"default:pending" json:"status"`
	TotalChunks      int       `gorm:"default:0" json:"total_chunks"`
	ProcessedChunks  int       `gorm:"default:0" json:"processed_chunks"`
	Error            string    `gorm:"type:text" json:"error"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type BotKnowledgeBinding struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BotID           string    `gorm:"not null;index" json:"bot_id"`
	KnowledgeBaseID string    `gorm:"not null;index" json:"kb_id"`
}

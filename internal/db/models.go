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
	Endpoint  string    `gorm:"not null" json:"endpoint"`
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

type Message struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	SessionKey string    `gorm:"not null;index" json:"session_key"`
	Role       string    `gorm:"not null" json:"role"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Tokens     int       `gorm:"default:0" json:"tokens"`
	Expired    bool      `gorm:"default:0" json:"expired"`
	CreatedAt  time.Time `json:"created_at"`
}

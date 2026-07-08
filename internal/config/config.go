// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package config

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type LLMProviderConfig struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"api_key"`
}

type MCPConfig struct {
	Name     string            `yaml:"name"`
	Endpoint string            `yaml:"endpoint"`
	Command  string            `yaml:"command"`
	Args     string            `yaml:"args"`
	Env      map[string]string `yaml:"env"`
}

type SkillConfig struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description"`
	SystemPrompt string `yaml:"system_prompt"`
	Tools        string `yaml:"tools"`
	Enabled      bool   `yaml:"enabled"`
}

type BotConfig struct {
	Name             string        `yaml:"name"`
	WecomBotID       string        `yaml:"wecom_bot_id"`
	WecomSecret      string        `yaml:"wecom_secret"`
	LLMProviderID    string        `yaml:"llm_provider_id"`
	LLMModel         string        `yaml:"llm_model"`
	LLMTemperature   float64       `yaml:"llm_temperature"`
	LLMMaxTokens     int           `yaml:"llm_max_tokens"`
	MaxSessionTokens int           `yaml:"max_session_tokens"`
	Enabled          bool          `yaml:"enabled"`
	Skills           []SkillConfig `yaml:"skills"`
}

type DatabaseConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	MaxOpenConns int   `yaml:"max_open_conns"`
}

type WeaviateConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Scheme    string `yaml:"scheme"`
	APIKey    string `yaml:"api_key"`
}

type EmbeddingConfig struct {
	ModelPath string `yaml:"model_path"`
	ModelURL  string `yaml:"model_url"`
	Enabled   bool   `yaml:"enabled"`
}

type KnowledgeBaseConfig struct {
	MaxFileSize       int64           `yaml:"max_file_size"`
	AllowedExtensions []string        `yaml:"allowed_extensions"`
	ChunkSize         int             `yaml:"chunk_size"`
	ChunkOverlap      int             `yaml:"chunk_overlap"`
	SearchDefaultTopK int             `yaml:"search_default_top_k"`
	SearchHybridAlpha float64         `yaml:"search_hybrid_alpha"`
	Embedding         EmbeddingConfig `yaml:"embedding"`
}

type Config struct {
	Database         DatabaseConfig      `yaml:"database"`
	LLMProviders     []LLMProviderConfig `yaml:"llm_providers"`
	MCPs             []MCPConfig         `yaml:"mcps"`
	Bots             []BotConfig         `yaml:"bots"`
	SkillsDir        string              `yaml:"skills_dir"`
	GoJudgeEndpoint  string              `yaml:"go_judge_endpoint"`
	Weaviate         WeaviateConfig      `yaml:"weaviate"`
	KnowledgeBase    KnowledgeBaseConfig `yaml:"knowledge_base"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.DSN == "" {
		cfg.Database.DSN = ".db/bots-nest.db?_journal_mode=WAL"
	}
	if cfg.Database.MaxOpenConns == 0 {
		cfg.Database.MaxOpenConns = 1
	}

	if cfg.LLMProviders == nil {
		cfg.LLMProviders = []LLMProviderConfig{}
	}
	if cfg.MCPs == nil {
		cfg.MCPs = []MCPConfig{}
	}
	if cfg.Bots == nil {
		cfg.Bots = []BotConfig{}
	}

	if cfg.KnowledgeBase.Embedding.ModelPath == "" {
		cfg.KnowledgeBase.Embedding.ModelPath = "data/embedding/models/all-MiniLM-L6-v2.Q5_K_M.gguf"
	}
	if cfg.KnowledgeBase.Embedding.ModelURL == "" {
		cfg.KnowledgeBase.Embedding.ModelURL = "https://huggingface.co/ashleyliu31/all-MiniLM-L6-v2-GGUF/resolve/main/all-MiniLM-L6-v2.Q5_K_M.gguf"
	}

	if cfg.SkillsDir == "" {
		cfg.SkillsDir = "skills"
	}
	if env := os.Getenv("SKILLS_DIR"); env != "" {
		cfg.SkillsDir = strings.TrimRight(env, "/")
	}

	return &cfg, nil
}

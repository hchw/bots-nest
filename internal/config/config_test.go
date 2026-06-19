// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	yamlContent := `
llm_providers:
  - name: "test"
    endpoint: "https://test.com/v1"
    api_key: "sk-test"

mcps:
  - name: "test-mcp"
    endpoint: "http://localhost:9090"

bots:
  - name: "test-bot"
    wecom_bot_id: "botid123"
    wecom_secret: "secret123"
    llm_provider_id: "test"
    llm_model: "gpt-4o"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if len(cfg.LLMProviders) != 1 {
		t.Errorf("期望 1 个 LLM Provider，得到 %d", len(cfg.LLMProviders))
	}
	if len(cfg.MCPs) != 1 {
		t.Errorf("期望 1 个 MCP，得到 %d", len(cfg.MCPs))
	}
	if len(cfg.Bots) != 1 {
		t.Errorf("期望 1 个 Bot，得到 %d", len(cfg.Bots))
	}

	if cfg.LLMProviders[0].Name != "test" {
		t.Errorf("期望 name=test，得到 %s", cfg.LLMProviders[0].Name)
	}
}

func TestLoadConfig_Empty(t *testing.T) {
	yamlContent := `{}`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Write([]byte(yamlContent))
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	if cfg.LLMProviders == nil {
		t.Error("LLMProviders 不应为 nil")
	}
	if cfg.MCPs == nil {
		t.Error("MCPs 不应为 nil")
	}
	if cfg.Bots == nil {
		t.Error("Bots 不应为 nil")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := Load("/tmp/nonexistent-config.yaml")
	if err == nil {
		t.Error("期望文件不存在错误")
	}
}

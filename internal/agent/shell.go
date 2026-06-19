// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package agent

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ShellExecutor struct {
	allowedCommands []string
	timeout         time.Duration
	maxOutputLen    int
}

func NewShellExecutor(allowed []string, timeout time.Duration, maxOutput int) *ShellExecutor {
	return &ShellExecutor{
		allowedCommands: allowed,
		timeout:         timeout,
		maxOutputLen:    maxOutput,
	}
}

func (e *ShellExecutor) isAllowed(command string) bool {
	if len(e.allowedCommands) == 0 {
		return true
	}
	cmdName := strings.Fields(command)[0]
	for _, allowed := range e.allowedCommands {
		if cmdName == allowed {
			return true
		}
	}
	return false
}

func (e *ShellExecutor) isInteractive(command string) bool {
	interactive := []string{"vim", "nano", "vi", "top", "less", "more", "bash", "sh", "zsh"}
	cmdName := strings.Fields(command)[0]
	for _, icmd := range interactive {
		if cmdName == icmd {
			return true
		}
	}
	return false
}

func (e *ShellExecutor) Execute(command string) (string, error) {
	if !e.isAllowed(command) {
		return "", fmt.Errorf("命令不在白名单中: %s", strings.Fields(command)[0])
	}

	if e.isInteractive(command) {
		return "", fmt.Errorf("交互式命令不允许: %s", command)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("执行失败: %w", err)
	}

	result := string(output)
	if len(result) > e.maxOutputLen {
		result = result[:e.maxOutputLen] + "\n... (输出已截断)"
	}

	return result, nil
}

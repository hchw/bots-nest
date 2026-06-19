// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package agent

import (
	"testing"
	"time"
)

func TestShellExecutor_AllowAll(t *testing.T) {
	exec := NewShellExecutor(nil, 5*time.Second, 1024)
	result, err := exec.Execute("echo hello")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "hello\n" {
		t.Errorf("期望 'hello\\n'，得到 %q", result)
	}
}

func TestShellExecutor_Interactive(t *testing.T) {
	exec := NewShellExecutor(nil, 5*time.Second, 1024)
	_, err := exec.Execute("vim")
	if err == nil {
		t.Error("期望交互式命令被拒绝")
	}
}

func TestShellExecutor_Whitelist(t *testing.T) {
	exec := NewShellExecutor([]string{"echo", "ls"}, 5*time.Second, 1024)
	_, err := exec.Execute("rm -rf /")
	if err == nil {
		t.Error("期望命令被白名单拒绝")
	}
}

func TestShellExecutor_OutputTruncation(t *testing.T) {
	exec := NewShellExecutor(nil, 5*time.Second, 5)
	result, err := exec.Execute("echo 'hello world long output'")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if len(result) > 30 {
		t.Errorf("输出应被截断，当前长度 %d", len(result))
	}
}

func TestShellExecutor_Timeout(t *testing.T) {
	exec := NewShellExecutor(nil, 100*time.Millisecond, 1024)
	_, err := exec.Execute("sleep 5")
	if err == nil {
		t.Error("期望超时错误")
	}
}

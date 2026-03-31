package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

var Shell = Info{
	Name:        "shell",
	Label:       "Shell 命令",
	PayloadHint: `{"commands": ["echo hello", "ls -la"]}`,
	Fn:          shellFn,
}

func shellFn(ctx context.Context, payload json.RawMessage) error {
	var p struct {
		Commands []string `json:"commands"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("解析 shell payload 失败: %w", err)
	}
	if len(p.Commands) == 0 {
		return fmt.Errorf("commands 为空")
	}

	for i, cmdStr := range p.Commands {
		cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		AppendOutput(ctx, fmt.Sprintf("$ %s\n", cmdStr))

		slog.InfoContext(ctx, "执行 shell 命令", "index", i, "cmd", cmdStr)
		if err := cmd.Run(); err != nil {
			errOut := strings.TrimSpace(stderr.String())
			if errOut != "" {
				AppendOutput(ctx, errOut+"\n")
			}
			return fmt.Errorf("命令[%d]执行失败: %s, stderr: %s", i, err, errOut)
		}

		if out := strings.TrimSpace(stdout.String()); out != "" {
			slog.InfoContext(ctx, "shell 输出", "index", i, "stdout", out)
			AppendOutput(ctx, out+"\n")
		}
		if errOut := strings.TrimSpace(stderr.String()); errOut != "" {
			AppendOutput(ctx, "[stderr] "+errOut+"\n")
		}
	}
	return nil
}

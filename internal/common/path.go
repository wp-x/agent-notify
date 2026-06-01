package common

import (
	"os"
	"path/filepath"
	"strings"
)

const DefaultBinaryPath = "agent-notify"

func ResolveBinaryPath(input string) string {
	input = strings.TrimSpace(input)
	if input != "" {
		return toUnixStylePath(input)
	}

	executablePath, err := os.Executable()
	if err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(executablePath); resolveErr == nil {
			return toUnixStylePath(resolved)
		}
		return toUnixStylePath(executablePath)
	}

	return DefaultBinaryPath
}

// toUnixStylePath converts backslashes to forward slashes (e.g., C:\Users\... -> C:/Users/...)
// This format works in cmd.exe, PowerShell, and Git Bash on Windows.
func toUnixStylePath(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

// Package doctor provides the diagnostics service for agent-notify.
// It checks the current notification setup and reports status.
package doctor

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
)

// OutputWriter handles output messages.
type OutputWriter interface {
	Writef(format string, args ...any)
}

// Service handles diagnostics for agent-notify.
type Service struct {
	claudeIntegration agentintegrations.Integration
	codexIntegration  agentintegrations.Integration
}

// NewService creates a new doctor service.
func NewService(opts ...Option) *Service {
	s := &Service{
		claudeIntegration: agentintegrations.NewClaudeIntegration(),
		codexIntegration:  agentintegrations.NewCodexIntegration(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Option configures the service.
type Option func(*Service)

// WithClaudeIntegration sets the Claude integration.
func WithClaudeIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.claudeIntegration = i }
}

// WithCodexIntegration sets the Codex integration.
func WithCodexIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.codexIntegration = i }
}

type DiagnosticStatus string

const (
	StatusInstalled          DiagnosticStatus = "installed"
	StatusAgentMissing       DiagnosticStatus = "agent_missing"
	StatusConfigMissing      DiagnosticStatus = "config_missing"
	StatusIntegrationMissing DiagnosticStatus = "integration_missing"
)

// DiagnosticsResult contains diagnostic results.
type DiagnosticsResult struct {
	ConfigPath              string
	ConfigExists            bool
	ClaudeInstalled         bool
	ClaudeHookInstalled     bool
	CodexInstalled          bool
	CodexHookInstalled      bool
	SystemNotifyAvailable   bool
	SystemNotifyName        string
	FeishuCLIReady          bool
	ClaudeFeishuEnabled     bool
	ClaudeSystemEnabled     bool
	ClaudeWechatWorkEnabled bool
	ClaudeDingTalkEnabled   bool
	ClaudeBarkEnabled       bool
	CodexFeishuEnabled      bool
	CodexSystemEnabled      bool
	CodexWechatWorkEnabled  bool
	CodexDingTalkEnabled    bool
	CodexBarkEnabled        bool
	ClaudeIntegrationStatus DiagnosticStatus
	CodexIntegrationStatus  DiagnosticStatus
}

// Run executes diagnostics and returns results.
func (s *Service) Run() (*DiagnosticsResult, error) {
	result := &DiagnosticsResult{}

	// Detect agents
	result.ClaudeInstalled = s.claudeIntegration.DetectInstalled()
	result.CodexInstalled = s.codexIntegration.DetectInstalled()

	// System notification detection
	result.SystemNotifyAvailable, result.SystemNotifyName = detectSystemNotification()

	// Config
	cfgPath, _ := config.DefaultPath()
	result.ConfigPath = cfgPath
	cfg, cfgLoadErr := config.Load(cfgPath)
	_, cfgErr := os.Stat(cfgPath)
	result.ConfigExists = cfgErr == nil

	// Claude hooks settings
	claudeSettingsPath, _ := s.claudeIntegration.SettingsPath("user")
	if claudeSettingsPath != "" {
		installed, err := s.claudeIntegration.IsHookInstalled(claudeSettingsPath)
		result.ClaudeHookInstalled = err == nil && installed
	}

	// Codex hooks settings
	codexSettingsPath, _ := s.codexIntegration.SettingsPath("user")
	if codexSettingsPath != "" {
		installed, err := s.codexIntegration.IsHookInstalled(codexSettingsPath)
		result.CodexHookInstalled = err == nil && installed
	}

	// Config values
	result.ClaudeFeishuEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Feishu.Enabled
	result.ClaudeSystemEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.System.Enabled
	result.ClaudeWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled
	result.ClaudeDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled
	result.ClaudeBarkEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Bark.Enabled
	result.CodexFeishuEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Feishu.Enabled
	result.CodexSystemEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.System.Enabled
	result.CodexWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.WechatWork.Enabled
	result.CodexDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.DingTalk.Enabled
	result.CodexBarkEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Bark.Enabled

	result.ClaudeIntegrationStatus = integrationStatus(result.ConfigExists, result.ClaudeInstalled, result.ClaudeHookInstalled)
	result.CodexIntegrationStatus = integrationStatus(result.ConfigExists, result.CodexInstalled, result.CodexHookInstalled)

	// Feishu CLI
	_, feishuCLIConfigErr := feishucli.ParseConfig()
	result.FeishuCLIReady = feishuCLIConfigErr == nil

	return result, nil
}

func integrationStatus(configExists, agentInstalled, integrationInstalled bool) DiagnosticStatus {
	if !agentInstalled {
		return StatusAgentMissing
	}
	if !configExists {
		return StatusConfigMissing
	}
	if !integrationInstalled {
		return StatusIntegrationMissing
	}
	return StatusInstalled
}

// Print outputs the diagnostics result.
func (s *Service) Print(output OutputWriter, result *DiagnosticsResult) {
	// Config path header
	output.Writef("配置文件: %s\n\n", result.ConfigPath)

	// Agent installation status table.
	// 列宽：Agent=14, 安装状态=10, 集成配置=16（emoji=2 列、中文=2 列）
	output.Writef("【Agent 安装状态】\n")
	output.Writef("+--------------+----------+----------------+\n")
	output.Writef("| Agent        | 安装状态 | 集成配置       |\n")
	output.Writef("+--------------+----------+----------------+\n")

	claudeInstallStatus := padRight("❌ 未安装", 8)
	if result.ClaudeInstalled {
		claudeInstallStatus = padRight("✅ 已安装", 8)
	}
	claudeHookStatus := padRight(diagnosticStatusLabel(result.ClaudeIntegrationStatus), 14)
	output.Writef("| %-12s | %s | %s |\n", "Claude Code", claudeInstallStatus, claudeHookStatus)

	codexInstallStatus := padRight("❌ 未安装", 8)
	if result.CodexInstalled {
		codexInstallStatus = padRight("✅ 已安装", 8)
	}
	codexNotifyStatus := padRight(diagnosticStatusLabel(result.CodexIntegrationStatus), 14)
	output.Writef("| %-12s | %s | %s |\n", "Codex", codexInstallStatus, codexNotifyStatus)

	output.Writef("+--------------+----------+----------------+\n")
	output.Writef("\n")

	// Notification channels table — 与 printCurrentNotifyConfig 保持一致的列宽。
	output.Writef("【通知渠道状态】\n")
	output.Writef("+--------------+------+------+----------+------+------+\n")
	output.Writef("| Agent        | 飞书 | 系统 | 企业微信 | 钉钉 | Bark |\n")
	output.Writef("+--------------+------+------+----------+------+------+\n")
	output.Writef("| %-12s |  %s  |  %s  |    %s    |  %s  |  %s  |\n", "Claude Code",
		boolIcon(result.ClaudeFeishuEnabled),
		boolIcon(result.ClaudeSystemEnabled),
		boolIcon(result.ClaudeWechatWorkEnabled),
		boolIcon(result.ClaudeDingTalkEnabled),
		boolIcon(result.ClaudeBarkEnabled),
	)
	output.Writef("| %-12s |  %s  |  %s  |    %s    |  %s  |  %s  |\n", "Codex",
		boolIcon(result.CodexFeishuEnabled),
		boolIcon(result.CodexSystemEnabled),
		boolIcon(result.CodexWechatWorkEnabled),
		boolIcon(result.CodexDingTalkEnabled),
		boolIcon(result.CodexBarkEnabled),
	)
	output.Writef("+--------------+------+------+----------+------+------+\n")
	output.Writef("\n")

	// System environment table — 列宽：检查项=20, 状态=10
	output.Writef("【系统环境】\n")
	output.Writef("+----------------------+------------+\n")
	output.Writef("| 检查项               | 状态       |\n")
	output.Writef("+----------------------+------------+\n")

	configStatus := padRight("❌ 不存在", 10)
	if result.ConfigExists {
		configStatus = padRight("✅ 已存在", 10)
	}
	output.Writef("| %s | %s |\n", padRight("配置文件", 20), configStatus)

	systemNotifyStatus := padRight("❌ 不可用", 10)
	if result.SystemNotifyAvailable {
		systemNotifyStatus = padRight("✅ 可用", 10)
	}
	output.Writef("| %s | %s |\n", padRight(result.SystemNotifyName, 20), systemNotifyStatus)

	feishuCLIStatus := padRight("❌ 未配置", 10)
	if result.FeishuCLIReady {
		feishuCLIStatus = padRight("✅ 已就绪", 10)
	}
	output.Writef("| %s | %s |\n", padRight("飞书 CLI", 20), feishuCLIStatus)

	output.Writef("+----------------------+------------+\n")
}

// boolIcon returns the ✅/❌ icon for a boolean status.
func boolIcon(enabled bool) string {
	if enabled {
		return "✅"
	}
	return "❌"
}

// detectSystemNotification checks if system notifications are available.
// Returns (available, displayName) where displayName is platform-specific.
func detectSystemNotification() (bool, string) {
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("osascript")
		return err == nil, "系统通知"
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil, "系统通知"
	case "windows":
		// PowerShell is always available on Windows
		return true, "系统通知"
	default:
		return false, "系统通知"
	}
}

// visualWidth calculates the visual width of a string, treating Chinese characters as 2 columns.
func visualWidth(s string) int {
	width := 0
	for _, r := range s {
		if utf8.RuneLen(r) > 1 {
			// Chinese and other wide characters
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// padRight pads a string to the target visual width.
func padRight(s string, targetWidth int) string {
	currentWidth := visualWidth(s)
	if currentWidth >= targetWidth {
		return s
	}
	padding := targetWidth - currentWidth
	return s + strings.Repeat(" ", padding)
}

func diagnosticStatusLabel(status DiagnosticStatus) string {
	switch status {
	case StatusInstalled:
		return "✅ 已安装"
	case StatusAgentMissing:
		return "❌ 未安装 Agent"
	case StatusConfigMissing:
		return "❌ 缺少配置"
	case StatusIntegrationMissing:
		return "❌ 未集成"
	default:
		return "❌ 未知"
	}
}

package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for agent-notify.
type Config struct {
	Version  int            `yaml:"version"`  // 配置版本号
	Agent    AgentConfig    `yaml:"agent"`    // Agent 安装配置
	Notify   NotifyConfig   `yaml:"notify"`   // 通知配置
	Behavior BehaviorConfig `yaml:"behavior"` // 行为配置
}

// AgentConfig holds configuration for supported agents.
type AgentConfig struct {
	ClaudeCode AgentTargetConfig `yaml:"claude_code"` // Claude Code 配置
	Codex      AgentTargetConfig `yaml:"codex"`       // Codex 配置
}

// AgentTargetConfig holds configuration for a specific agent.
type AgentTargetConfig struct {
	Enabled      bool   `yaml:"enabled"`       // 是否启用该 Agent 的通知
	InstallScope string `yaml:"install_scope"` // 安装范围: user 或 project
}

// NotifyConfig holds notification configuration for all agents.
type NotifyConfig struct {
	ClaudeCode AgentNotifyConfig `yaml:"claude_code"` // Claude Code 通知配置
	Codex      AgentNotifyConfig `yaml:"codex"`       // Codex 通知配置
}

// AgentNotifyConfig holds notification configuration for a single agent.
type AgentNotifyConfig struct {
	Events   []string       `yaml:"events,omitempty"` // 通知事件列表，如: permission_required, input_required, run_completed, run_failed
	Channels ChannelsConfig `yaml:"channels"`         // 通知渠道配置
}

// ChannelsConfig holds configuration for notification channels.
type ChannelsConfig struct {
	Feishu     ChannelConfig           `yaml:"feishu"`      // 飞书通知配置
	System     ChannelConfig           `yaml:"system"`      // 系统通知配置
	WechatWork WechatWorkChannelConfig `yaml:"wechat_work"` // 企业微信通知配置
	DingTalk   DingTalkChannelConfig   `yaml:"dingtalk"`    // 钉钉通知配置
	Bark       BarkChannelConfig       `yaml:"bark"`        // Bark 通知配置
}

// ChannelConfig holds configuration for a single notification channel.
type ChannelConfig struct {
	Enabled bool `yaml:"enabled"` // 是否启用该通知渠道
}

// WechatWorkChannelConfig holds configuration for WeChat Work (企业微信) webhook notifications.
type WechatWorkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用企业微信通知
	WebhookURL string `yaml:"webhook_url"` // 群机器人 Webhook URL
}

// DingTalkChannelConfig holds configuration for DingTalk (钉钉) webhook notifications.
type DingTalkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用钉钉通知
	WebhookURL string `yaml:"webhook_url"` // 群机器人 Webhook URL
}

// BarkChannelConfig holds configuration for Bark webhook notifications.
type BarkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用 Bark 通知
	WebhookURL string `yaml:"webhook_url"` // Bark 推送 URL
}

// BehaviorConfig holds behavior configuration.
type BehaviorConfig struct {
	DedupeSeconds      int    `yaml:"dedupe_seconds"`       // 去重时间窗口（秒），同一事件在此时间内不重复发送
	SendTimeoutSeconds int    `yaml:"send_timeout_seconds"` // 发送超时时间（秒）
	Locale             string `yaml:"locale"`               // 语言设置，如: zh-CN, en-US
}

func Default() Config {
	allEvents := []string{"permission_required", "input_required", "run_completed", "run_failed"}
	// Codex hooks 当前可靠支持的两个事件
	codexEvents := []string{"permission_required", "run_completed"}

	return Config{
		Version: 1,
		Agent: AgentConfig{
			ClaudeCode: AgentTargetConfig{
				Enabled:      true,
				InstallScope: "user",
			},
			Codex: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
		},
		Notify: NotifyConfig{
			ClaudeCode: AgentNotifyConfig{
				Events: append([]string(nil), allEvents...),
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: true},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
				},
			},
			Codex: AgentNotifyConfig{
				Events: append([]string(nil), codexEvents...),
				Channels: ChannelsConfig{
					System:     ChannelConfig{Enabled: false},
					Feishu:     ChannelConfig{Enabled: false},
					WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
					DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
					Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
				},
			},
		},
		Behavior: BehaviorConfig{
			DedupeSeconds:      60,
			SendTimeoutSeconds: 5,
			Locale:             "zh-CN",
		},
	}
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "config.yaml"), nil
}

func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "state.json"), nil
}

func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "agent-notify.log"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}

	// 先解析到空结构体，避免默认值干扰
	cfg := Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// 填充默认值（仅对未设置的字段）
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Agent.ClaudeCode.InstallScope == "" {
		cfg.Agent.ClaudeCode.InstallScope = "user"
	}
	if cfg.Agent.Codex.InstallScope == "" {
		cfg.Agent.Codex.InstallScope = "user"
	}
	if cfg.Behavior.DedupeSeconds == 0 {
		cfg.Behavior.DedupeSeconds = 60
	}
	if cfg.Behavior.SendTimeoutSeconds == 0 {
		cfg.Behavior.SendTimeoutSeconds = 5
	}
	if cfg.Behavior.Locale == "" {
		cfg.Behavior.Locale = "zh-CN"
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

package setup

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
)

const (
	agentClaude     = "claude"
	agentCodex      = "codex"
	channelSystem   = "system"
	channelFeishu   = "feishu"
	channelWXWork   = "wechat-work"
	channelDingTalk = "dingtalk"
	channelBark     = "bark"
	installScopeUsr = "user"
	installScopePrj = "project"
)

var channelOptions = []PromptOption{
	{Label: "系统通知", Value: channelSystem},
	{Label: "飞书", Value: channelFeishu},
	{Label: "企业微信", Value: channelWXWork},
	{Label: "钉钉", Value: channelDingTalk},
	{Label: "Bark", Value: channelBark},
}

type channelSelection struct {
	System     bool
	Feishu     bool
	WechatWork bool
	DingTalk   bool
	Bark       bool
}

func (c channelSelection) hasAny() bool {
	return c.System || c.Feishu || c.WechatWork || c.DingTalk || c.Bark
}

type configureAgentRequest struct {
	ctx        context.Context
	prompter   Prompter
	output     OutputWriter
	cfg        config.Config
	agent      string
	channels   channelSelection
	events     []string
	binaryPath string
}

type configuredAgent struct {
	cfg          config.Config
	settingsPath string
}

func (s *Service) selectAgent(prompter Prompter, cfg config.Config) (string, error) {
	agentOptions, defaultAgent := s.agentOptions(cfg)
	if len(agentOptions) == 0 {
		return "", errors.New("未检测到 Claude Code 或 Codex，请先安装其中一个")
	}
	if defaultAgent == "" {
		defaultAgent = agentOptions[0].Value
	}
	return prompter.Select("选择要配置的 Agent", agentOptions, defaultAgent)
}

func (s *Service) agentOptions(cfg config.Config) ([]PromptOption, string) {
	var options []PromptOption
	var defaultAgent string
	if s.claudeIntegration.DetectInstalled() {
		options = append(options, PromptOption{Label: "Claude Code", Value: agentClaude})
		if cfg.Agent.ClaudeCode.Enabled {
			defaultAgent = agentClaude
		}
	}
	if s.codexIntegration.DetectInstalled() {
		options = append(options, PromptOption{Label: "Codex", Value: agentCodex})
		if cfg.Agent.Codex.Enabled && defaultAgent == "" {
			defaultAgent = agentCodex
		}
	}
	return options, defaultAgent
}

func promptChannelSelection(prompter Prompter, channels config.ChannelsConfig) (channelSelection, error) {
	choices, err := prompter.MultiSelect("启用通知渠道", channelOptions, currentChannelValues(channels))
	if err != nil {
		return channelSelection{}, err
	}
	return channelSelectionFromChoices(choices), nil
}

func currentChannelValues(channels config.ChannelsConfig) []string {
	values := make([]string, 0, len(channelOptions))
	if channels.System.Enabled {
		values = append(values, channelSystem)
	}
	if channels.Feishu.Enabled {
		values = append(values, channelFeishu)
	}
	if channels.WechatWork.Enabled {
		values = append(values, channelWXWork)
	}
	if channels.DingTalk.Enabled {
		values = append(values, channelDingTalk)
	}
	if channels.Bark.Enabled {
		values = append(values, channelBark)
	}
	return values
}

func channelSelectionFromChoices(choices []string) channelSelection {
	return channelSelection{
		System:     slices.Contains(choices, channelSystem),
		Feishu:     slices.Contains(choices, channelFeishu),
		WechatWork: slices.Contains(choices, channelWXWork),
		DingTalk:   slices.Contains(choices, channelDingTalk),
		Bark:       slices.Contains(choices, channelBark),
	}
}

func promptEvents(prompter Prompter, agent string, currentEvents []string) ([]string, error) {
	return prompter.MultiSelect("通知事件", eventOptionsForAgent(agent), currentEvents)
}

func eventOptionsForAgent(agent string) []PromptOption {
	if agent == agentClaude {
		return claudeEventOptions
	}
	return codexEventOptions
}

func channelsForAgent(cfg config.Config, agent string) config.ChannelsConfig {
	if agent == agentClaude {
		return cfg.Notify.ClaudeCode.Channels
	}
	return cfg.Notify.Codex.Channels
}

func eventsForAgent(cfg config.Config, agent string) []string {
	if agent == agentClaude {
		return cfg.Notify.ClaudeCode.Events
	}
	return cfg.Notify.Codex.Events
}

func (s *Service) configureAgent(req configureAgentRequest) (configuredAgent, error) {
	switch req.agent {
	case agentClaude:
		return s.configureClaude(req)
	case agentCodex:
		return s.configureCodex(req)
	default:
		return configuredAgent{}, fmt.Errorf("unsupported agent: %s", req.agent)
	}
}

func (s *Service) configureClaude(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.ClaudeCode.Channels = applyChannelSelection(next.Notify.ClaudeCode.Channels, req.channels)
	next.Notify.ClaudeCode.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	channels, err := promptWebhookURLs(req.prompter, next.Notify.ClaudeCode.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.ClaudeCode.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.ClaudeCode.InstallScope)
	settingsPath, err := s.claudeIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 claude settings 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.claudeIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 claude hooks 失败: %w", err)
	}
	req.output.Writef("claude hooks 安装: %s\n", settingsPath)
	next.Agent.ClaudeCode.InstallScope = agentScope
	next.Agent.ClaudeCode.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func (s *Service) configureCodex(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.Codex.Channels = applyChannelSelection(next.Notify.Codex.Channels, req.channels)
	next.Notify.Codex.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	channels, err := promptWebhookURLs(req.prompter, next.Notify.Codex.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.Codex.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.Codex.InstallScope)
	settingsPath, err := s.codexIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("获取 codex hooks 路径失败: %w", err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.codexIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("安装 codex hooks 失败: %w", err)
	}
	req.output.Writef("codex hooks 安装: %s\n", settingsPath)
	req.output.Writef("提示: 请在 codex 内运行 /hooks 完成 trust 审核\n")
	next.Agent.Codex.InstallScope = agentScope
	next.Agent.Codex.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func applyChannelSelection(channels config.ChannelsConfig, selection channelSelection) config.ChannelsConfig {
	next := channels
	next.System.Enabled = selection.System
	next.Feishu.Enabled = selection.Feishu
	next.WechatWork.Enabled = selection.WechatWork
	next.DingTalk.Enabled = selection.DingTalk
	next.Bark.Enabled = selection.Bark
	return next
}

func (s *Service) prepareSelectedChannels(ctx context.Context, selection channelSelection) error {
	if !selection.Feishu {
		return nil
	}
	if err := s.prepareFeishu(ctx); err != nil {
		return fmt.Errorf("飞书初始化失败: %w", err)
	}
	return nil
}

func promptWebhookURLs(
	prompter Prompter,
	channels config.ChannelsConfig,
	selection channelSelection,
) (config.ChannelsConfig, error) {
	next := channels
	if selection.WechatWork {
		webhookURL, err := prompter.Input("企业微信群机器人 Webhook URL", next.WechatWork.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.WechatWork.WebhookURL = webhookURL
	}
	if selection.DingTalk {
		webhookURL, err := prompter.Input("钉钉群机器人 Webhook URL", next.DingTalk.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.DingTalk.WebhookURL = webhookURL
	}
	if selection.Bark {
		webhookURL, err := prompter.Input("Bark Webhook URL", next.Bark.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.Bark.WebhookURL = webhookURL
	}
	return next, nil
}

func normalizedInstallScope(scope string) string {
	if scope == installScopePrj {
		return installScopePrj
	}
	return installScopeUsr
}

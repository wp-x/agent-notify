package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/config"
)

func runTestDingTalk(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL := dingTalkURLFromConfig(cfg)
	if webhookURL == "" {
		return fmt.Errorf("未配置钉钉 Webhook URL，请先运行配置向导")
	}

	svc := tester.NewService()
	result, err := svc.TestDingTalk(ctx, webhookURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitDingTalk(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL, err := prompter.Input("钉钉群机器人 Webhook URL", dingTalkURLFromConfig(cfg))
	if err != nil {
		return err
	}

	cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled = true
	cfg.Notify.ClaudeCode.Channels.DingTalk.WebhookURL = webhookURL
	cfg.Notify.Codex.Channels.DingTalk.Enabled = true
	cfg.Notify.Codex.Channels.DingTalk.WebhookURL = webhookURL

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Fprintln(streams.Stdout, "✅ 钉钉 Webhook 配置完成")
	fmt.Fprintf(streams.Stdout, "配置文件: %s\n", path)
	return nil
}

func dingTalkURLFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.DingTalk.WebhookURL != "" {
		return cfg.Notify.ClaudeCode.Channels.DingTalk.WebhookURL
	}
	return cfg.Notify.Codex.Channels.DingTalk.WebhookURL
}

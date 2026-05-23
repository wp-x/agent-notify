package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
)

const banner = `
╔════════════════════════════════════════════════════════════════╗
║            █████╗  ██████╗ ███████╗███╗   ██╗████████╗         ║
║           ██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝         ║
║           ███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║            ║
║           ██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║            ║
║           ██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║            ║
║           ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝            ║
║                        Agent Notify                            ║
║         Claude Code / Codex Notification Setup Tool            ║
╚════════════════════════════════════════════════════════════════╝
`

func runMenu(ctx context.Context, streams Streams) error {
	prompter, err := newPrompter(streams)
	if err != nil {
		return err
	}

	// 只在首次显示 banner
	renderBanner(streams)

	for {
		choice, err := prompter.Select("", []PromptOption{
			{Label: "Agent通知配置", Value: "init"},
			{Label: "消息渠道配置", Value: "channels"},
			{Label: "测试通知", Value: "test"},
			{Label: "环境诊断", Value: "doctor"},
			{Label: "查看配置", Value: "view"},
			{Label: "清理配置", Value: "clean"},
			{Label: "退出", Value: "quit"},
		}, "init")
		if err != nil {
			if errors.Is(err, ErrCancelled) {
				return nil // Ctrl+C 退出程序
			}
			return err
		}

		switch choice {
		case "init":
			if err := runInitFlow(ctx, streams, prompter, "", "", common.ResolveBinaryPath("")); err != nil {
				if errors.Is(err, ErrCancelled) {
					fmt.Fprintln(streams.Stdout) // 仅换行，不显示错误
				} else {
					fmt.Fprintf(streams.Stdout, "\n❌ 配置失败: %v\n\n", err)
				}
			} else {
				fmt.Fprint(streams.Stdout, "\n✅ 配置完成\n\n")
			}
		case "channels":
			if err := runChannelsMenu(ctx, streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n❌ 配置失败: %v\n\n", err)
				}
			}
		case "test":
			if err := runTestMenu(ctx, streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n❌ 测试失败: %v\n\n", err)
				}
			}
		case "doctor":
			if err := runDoctor(streams); err != nil {
				fmt.Fprintf(streams.Stdout, "\n❌ 诊断失败: %v\n\n", err)
			}
		case "view":
			if err := printCurrentNotifyConfig(streams); err != nil {
				fmt.Fprintf(streams.Stdout, "\n❌ 读取配置失败: %v\n\n", err)
			}
		case "clean":
			if err := runCleanConfig(streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n❌ 清理失败: %v\n\n", err)
				}
			}
		case "quit":
			return nil
		}
	}
}

func runTestMenu(ctx context.Context, streams Streams, prompter Prompter) error {
	choice, err := prompter.Select("测试通知", []PromptOption{
		{Label: "飞书", Value: "feishu"},
		{Label: "系统通知", Value: "system"},
		{Label: "企业微信", Value: "wechat-work"},
		{Label: "返回", Value: "back"},
	}, "feishu")
	if err != nil {
		return err
	}

	switch choice {
	case "feishu":
		return runTestFeishu(ctx, streams)
	case "system":
		return runTestSystem(ctx, streams)
	case "wechat-work":
		return runTestWechatWork(ctx, streams)
	default:
		return nil
	}
}

func runChannelsMenu(ctx context.Context, streams Streams, prompter Prompter) error {
	for {
		choice, err := prompter.Select("消息渠道配置", []PromptOption{
			{Label: "初始化飞书", Value: "feishu-init"},
			{Label: "初始化企业微信", Value: "wechatwork-init"},
			{Label: "返回", Value: "back"},
		}, "feishu-init")
		if err != nil {
			return err
		}

		switch choice {
		case "feishu-init":
			if _, err := feishucli.Reinitialize(ctx); err != nil {
				return err
			}
			fmt.Fprintln(streams.Stdout, "✅ 飞书 CLI 初始化完成")
		case "wechatwork-init":
			if err := runInitWechatWork(streams, prompter); err != nil {
				return err
			}
		case "back":
			return nil
		}
	}
}

func runCleanConfig(streams Streams, prompter Prompter) error {
	confirm, err := prompter.Confirm("确认清理所有配置？", false)
	if err != nil {
		return err
	}
	if !confirm {
		fmt.Fprintln(streams.Stdout, "已取消")
		return nil
	}

	// 清理 agent-notify 配置
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除配置文件失败: %w", err)
	}

	// 清理状态文件
	statePath, err := config.StatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除状态文件失败: %w", err)
	}

	// 清理日志文件
	logPath, err := config.LogPath()
	if err != nil {
		return err
	}
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除日志文件失败: %w", err)
	}

	// 保存一个干净的默认配置（所有通知都关闭）
	defaultCfg := config.Default()
	// Clear ClaudeCode channel toggles and events
	defaultCfg.Notify.ClaudeCode.Channels.Feishu.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.System.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.WechatWork.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL = ""
	defaultCfg.Notify.ClaudeCode.Events = nil
	// Clear Codex channel toggles
	defaultCfg.Notify.Codex.Channels.Feishu.Enabled = false
	defaultCfg.Notify.Codex.Channels.System.Enabled = false
	defaultCfg.Notify.Codex.Channels.WechatWork.Enabled = false
	defaultCfg.Notify.Codex.Channels.WechatWork.WebhookURL = ""
	defaultCfg.Notify.Codex.Events = nil
	if err := config.Save(cfgPath, defaultCfg); err != nil {
		return fmt.Errorf("保存默认配置失败: %w", err)
	}

	fmt.Fprintln(streams.Stdout, "✅ 配置已清理，下次配置时需要重新初始化飞书")
	return nil
}

func renderBanner(streams Streams) {
	fmt.Fprint(streams.Stdout, banner)
	fmt.Fprintf(streams.Stdout, "  Version: %s  |  https://github.com/hellolib/agent-notify\n\n", Version)
}

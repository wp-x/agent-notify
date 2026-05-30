package agenthooks

import (
	"context"
	"fmt"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

func Dispatch(ctx context.Context, cfg config.Config, statePath, logPath string, msg notify.Message) error {
	store := state.NewStore(statePath)
	senders := buildSenders(cfg, msg)
	if len(senders) == 0 {
		return state.AppendLog(logPath, fmt.Sprintf("no sender enabled for event=%s", msg.Event))
	}

	dispatcher := notify.NewDispatcher(store, time.Duration(cfg.Behavior.DedupeSeconds)*time.Second, senders...)
	timeout := time.Duration(cfg.Behavior.SendTimeoutSeconds) * time.Second
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := dispatcher.SendAll(sendCtx, msg); err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("dispatch error event=%s session=%s err=%v", msg.Event, msg.SessionID, err))
	}

	return nil
}

func buildSenders(cfg config.Config, msg notify.Message) []notify.Sender {
	var senders []notify.Sender

	notifyCfg := cfg.Notify.ClaudeCode
	if msg.Agent == "codex" {
		notifyCfg = cfg.Notify.Codex
	}

	if !contains(notifyCfg.Events, msg.Event) {
		return senders
	}

	if notifyCfg.Channels.System.Enabled {
		senders = append(senders, notify.NewSystemSender(notify.DefaultRunner))
	}
	if notifyCfg.Channels.Feishu.Enabled {
		senders = append(senders, notify.NewDefaultFeishuSender())
	}
	if notifyCfg.Channels.WechatWork.Enabled && notifyCfg.Channels.WechatWork.WebhookURL != "" {
		senders = append(senders, notify.NewWechatWorkSender(notifyCfg.Channels.WechatWork.WebhookURL))
	}
	if notifyCfg.Channels.DingTalk.Enabled && notifyCfg.Channels.DingTalk.WebhookURL != "" {
		senders = append(senders, notify.NewDingTalkSender(notifyCfg.Channels.DingTalk.WebhookURL))
	}
	if notifyCfg.Channels.Bark.Enabled && notifyCfg.Channels.Bark.WebhookURL != "" {
		senders = append(senders, notify.NewBarkSender(notifyCfg.Channels.Bark.WebhookURL))
	}

	return senders
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DingTalkSender sends notifications via DingTalk (钉钉) group bot webhook.
type DingTalkSender struct {
	webhookURL string
	httpClient *http.Client
}

// NewDingTalkSender creates a new DingTalkSender with the given webhook URL.
func NewDingTalkSender(webhookURL string) *DingTalkSender {
	return &DingTalkSender{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *DingTalkSender) Name() string { return "dingtalk" }

// Send sends a notification message via DingTalk webhook using the markdown message type.
func (s *DingTalkSender) Send(ctx context.Context, msg Message) error {
	if s.webhookURL == "" {
		return fmt.Errorf("dingtalk: webhook_url is empty")
	}

	title, content := s.buildMarkdown(msg)
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"title": title,
			"text":  content,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("dingtalk: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("dingtalk: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("dingtalk: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("dingtalk: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.ErrCode != 0 {
		return fmt.Errorf("dingtalk: api error code=%d msg=%s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// buildMarkdown constructs the markdown title and content for a DingTalk message.
func (s *DingTalkSender) buildMarkdown(msg Message) (string, string) {
	emoji := eventEmoji(msg.Event)
	eventName := eventDisplayName(msg.Event)
	levelBadge := dingTalkLevelBadge(msg.Event)
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	footerText := "🤖 Agent Notify"
	if msg.Agent == "codex" {
		footerText = "🤖 Codex Agent Notify"
	}

	title := fmt.Sprintf("%s %s", emoji, msg.Title)

	content := fmt.Sprintf("# %s %s\n\n", emoji, msg.Title)
	content += fmt.Sprintf("%s **%s**\n\n", levelBadge, eventName)
	content += "---\n\n"
	content += fmt.Sprintf("> **时间**：%s\n\n", timestamp)
	content += fmt.Sprintf("> **消息内容**：%s\n\n", msg.Body)
	if msg.Workspace != "" && msg.Agent != "codex" {
		content += fmt.Sprintf("> **工作目录**：`%s`\n\n", msg.Workspace)
	}
	content += "---\n\n"
	content += fmt.Sprintf("> %s", footerText)

	return title, content
}

// dingTalkLevelBadge returns a colored emoji badge approximating Feishu's header color.
// 钉钉 markdown 不支持彩色 header，用色块 emoji 模拟事件等级。
func dingTalkLevelBadge(event string) string {
	switch event {
	case "permission_required":
		return "🟠"
	case "input_required":
		return "🔵"
	case "run_completed":
		return "🟢"
	case "run_failed":
		return "🔴"
	default:
		return "🟣"
	}
}

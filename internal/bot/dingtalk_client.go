// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	dtclient "github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

type DingTalkClient struct {
	clientID     string
	clientSecret string

	streamClient *dtclient.StreamClient
	cancelFunc   context.CancelFunc

	msgHandler     func(msg *Message)
	statusCallback func(status string)
	status         string

	translator Translator

	mu     sync.Mutex
	stopCh chan struct{}
	started bool
}

type dingtalkReplyBody struct {
	MsgType string          `json:"msgtype"`
	Text    *dingtalkText   `json:"text,omitempty"`
	Markdown *dingtalkMarkdown `json:"markdown,omitempty"`
}

type dingtalkText struct {
	Content string `json:"content"`
}

type dingtalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

func NewDingTalkClient(clientID, clientSecret string) *DingTalkClient {
	return &DingTalkClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		translator:   NewDingTalkTranslator(),
		stopCh:       make(chan struct{}),
	}
}

func (d *DingTalkClient) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		return nil
	}

	d.notifyStatus("connecting")

	d.streamClient = dtclient.NewStreamClient(
		dtclient.WithAppCredential(dtclient.NewAppCredentialConfig(d.clientID, d.clientSecret)),
		dtclient.WithAutoReconnect(true),
	)

	d.streamClient.RegisterChatBotCallbackRouter(d.onMessageReceived)

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelFunc = cancel

	if err := d.streamClient.Start(ctx); err != nil {
		d.notifyStatus("error")
		return fmt.Errorf("钉钉 Stream 连接失败: %w", err)
	}

	d.started = true
	d.notifyStatus("connected")
	log.Printf("钉钉机器人 %s 已连接", d.clientID)

	go d.watchDisconnect()

	return nil
}

func (d *DingTalkClient) watchDisconnect() {
	<-d.stopCh
}

func (d *DingTalkClient) onMessageReceived(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	raw, _ := json.Marshal(data)
	parsed, err := d.translator.ParseIncoming(raw)
	if err != nil {
		log.Printf("钉钉消息转换失败: %v", err)
		return []byte(""), nil
	}
	if parsed == nil {
		return []byte(""), nil
	}

	if d.msgHandler != nil {
		d.msgHandler(parsed)
	}

	return []byte(""), nil
}

func (d *DingTalkClient) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.started {
		return
	}

	close(d.stopCh)

	if d.cancelFunc != nil {
		d.cancelFunc()
	}
	if d.streamClient != nil {
		d.streamClient.Close()
	}

	d.started = false
	d.notifyStatus("disconnected")
	log.Printf("钉钉机器人 %s 已断开", d.clientID)
}

func (d *DingTalkClient) Status() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.status
}

func (d *DingTalkClient) SetStatusCallback(cb func(status string)) {
	d.statusCallback = cb
}

func (d *DingTalkClient) SetMessageHandler(handler func(msg *Message)) {
	d.msgHandler = handler
}

func (d *DingTalkClient) Connected() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.started
}

func (d *DingTalkClient) Translator() Translator {
	return d.translator
}

func (d *DingTalkClient) SendReply(replyToken string, content string) error {
	body := dingtalkReplyBody{
		MsgType: "text",
		Text: &dingtalkText{
			Content: content,
		},
	}
	return d.postToWebhook(replyToken, body)
}

func (d *DingTalkClient) SendStreamChunk(reqID string, _ string, content string, finish bool) error {
	if finish && content != "" {
		return d.SendReply(reqID, content)
	}
	return nil
}

func (d *DingTalkClient) SendActiveMsg(_ string, _ string, _ int, _ string) error {
	return fmt.Errorf("钉钉 Stream 模式不支持主动推送消息")
}

func (d *DingTalkClient) postToWebhook(webhookURL string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("钉钉回复序列化失败: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("钉钉回复发送失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("钉钉回复返回非 200: %d", resp.StatusCode)
	}

	return nil
}

func (d *DingTalkClient) notifyStatus(status string) {
	d.status = status
	if d.statusCallback != nil {
		d.statusCallback(status)
	}
}

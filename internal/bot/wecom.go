// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (c) 2026 hchw

package bot

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hchw/bots-nest/internal/knowledge"
)

const wecomWSURL = "wss://openws.work.weixin.qq.com"

const (
	cmdSubscribe      = "aibot_subscribe"
	cmdMsgCallback    = "aibot_msg_callback"
	cmdEventCallback  = "aibot_event_callback"
	cmdRespondMsg     = "aibot_respond_msg"
	cmdRespondWelcome = "aibot_respond_welcome_msg"
)

type WeComMessage struct {
	Cmd     string       `json:"cmd"`
	Headers WeComHeaders `json:"headers"`
	Body    WeComMsgBody `json:"body"`
}

type WeComHeaders struct {
	ReqID string `json:"req_id"`
}

type WeComMsgBody struct {
	MsgID       string     `json:"msgid"`
	AibotID     string     `json:"aibotid"`
	ChatType    string     `json:"chattype"`
	From        *WeComFrom `json:"from"`
	ChatID      string     `json:"chatid,omitempty"`
	MsgType     string     `json:"msgtype"`
	Text        *WeComText `json:"text,omitempty"`
	ResponseURL string     `json:"response_url,omitempty"`
	EventType   string     `json:"event_type,omitempty"`
}

type WeComFrom struct {
	UserID string `json:"userid"`
}

type WeComText struct {
	Content string `json:"content"`
}

type WeComOutgoingMsg struct {
	Cmd     string            `json:"cmd"`
	Headers WeComHeaders      `json:"headers"`
	Body    WeComOutgoingBody `json:"body"`
}

type WeComOutgoingBody struct {
	MsgType string       `json:"msgtype"`
	Text    *WeComText   `json:"text,omitempty"`
	Stream  *WeComStream `json:"stream,omitempty"`
}

type WeComStream struct {
	ID      string `json:"id,omitempty"`
	Finish  bool   `json:"finish"`
	Content string `json:"content"`
}

type WeComClient struct {
	botID  string
	secret string

	conn           *websocket.Conn
	connMutex      sync.Mutex
	connected      bool
	msgHandler     func(msg *WeComMessage)
	stopChan       chan struct{}
	closeOnce      sync.Once
	wg             sync.WaitGroup
	statusCallback func(status string)

	lastSendTime    time.Time
	minSendInterval time.Duration
}

func NewWeComClient(botID, secret string) *WeComClient {
	return &WeComClient{
		botID:           botID,
		secret:          secret,
		stopChan:        make(chan struct{}),
		minSendInterval: 150 * time.Millisecond,
	}
}

func (w *WeComClient) SetMessageHandler(handler func(msg *WeComMessage)) {
	w.msgHandler = handler
}

func (w *WeComClient) SetStatusCallback(cb func(status string)) {
	w.statusCallback = cb
}

func (w *WeComClient) Connect() error {
	w.notifyStatus("connecting")

	if err := w.connect(); err != nil {
		w.notifyStatus("error")
		return err
	}

	w.notifyStatus("connected")
	return nil
}

func (w *WeComClient) connect() error {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()

	if w.connected {
		return nil
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial(wecomWSURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket 连接失败: %w", err)
	}

	subReq := map[string]interface{}{
		"cmd": cmdSubscribe,
		"headers": map[string]string{
			"req_id": fmt.Sprintf("sub_%d", time.Now().UnixNano()),
		},
		"body": map[string]string{
			"bot_id": w.botID,
			"secret": w.secret,
		},
	}
	if err := conn.WriteJSON(subReq); err != nil {
		conn.Close()
		return fmt.Errorf("订阅失败: %w", err)
	}

	w.conn = conn
	w.connected = true
	log.Printf("机器人 %s WebSocket 已连接并订阅", w.botID)

	w.wg.Add(1)
	go w.readLoop()
	return nil
}

func (w *WeComClient) notifyStatus(status string) {
	if w.statusCallback != nil {
		w.statusCallback(status)
	}
}

func (w *WeComClient) readLoop() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stopChan:
			return
		default:
		}

		_, message, err := w.conn.ReadMessage()
		if err != nil {
			log.Printf("机器人 %s WebSocket 读取错误: %v", w.botID, err)
			w.connMutex.Lock()
			w.connected = false
			w.connMutex.Unlock()
			w.notifyStatus("disconnected")

			select {
			case <-w.stopChan:
				return
			default:
			}
			time.Sleep(3 * time.Second)

			w.connMutex.Lock()
			stopped := false
			select {
			case <-w.stopChan:
				stopped = true
			default:
			}
			w.connMutex.Unlock()
			if stopped {
				return
			}

			if err := w.Connect(); err != nil {
				log.Printf("机器人 %s 重连失败: %v", w.botID, err)
				continue
			}
			return
		}

		// Check for API error/success responses (they have errcode, no cmd field)
		var rawMap map[string]interface{}
		if json.Unmarshal(message, &rawMap) == nil {
			if errcode, ok := rawMap["errcode"]; ok {
				code := int(errcode.(float64))
				if code != 0 {
					log.Printf("机器人 %s 发送消息返回错误: %s", w.botID, string(message))
				}
				continue
			}
		}

		var msg WeComMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("机器人 %s 消息解析失败: %v", w.botID, err)
			continue
		}

		switch msg.Cmd {
		case cmdMsgCallback:
			log.Printf("机器人 %s 收到消息: %s", w.botID, string(message))
			if w.msgHandler != nil {
				w.msgHandler(&msg)
			}
		case cmdEventCallback:
			log.Printf("机器人 %s 收到事件: %s", w.botID, msg.Body.EventType)
		default:
			log.Printf("机器人 %s 未知命令: cmd=%s, 原始消息: %s", w.botID, msg.Cmd, string(message))
		}
	}
}

func (w *WeComClient) SendReply(reqID string, content string) error {
	return w.SendStreamChunk(reqID, fmt.Sprintf("s_%d", time.Now().UnixNano()), content, true)
}

func (w *WeComClient) SendStreamChunk(reqID, streamID, content string, finish bool) error {
	msg := WeComOutgoingMsg{
		Cmd:     cmdRespondMsg,
		Headers: WeComHeaders{ReqID: reqID},
		Body: WeComOutgoingBody{
			MsgType: "stream",
			Stream: &WeComStream{
				ID:      streamID,
				Finish:  finish,
				Content: content,
			},
		},
	}
	return w.writeJSON(msg)
}

func (w *WeComClient) SendWelcome(reqID string, content string) error {
	msg := WeComOutgoingMsg{
		Cmd:     cmdRespondWelcome,
		Headers: WeComHeaders{ReqID: reqID},
		Body: WeComOutgoingBody{
			MsgType: "text",
			Text:    &WeComText{Content: content},
		},
	}
	return w.writeJSON(msg)
}

func (w *WeComClient) writeJSON(v interface{}) error {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	if w.conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}

	if !w.lastSendTime.IsZero() {
		elapsed := time.Since(w.lastSendTime)
		if elapsed < w.minSendInterval {
			time.Sleep(w.minSendInterval - elapsed)
		}
	}

	data, _ := json.Marshal(v)
	log.Printf("[DEBUG] 机器人 %s 发送消息: %s", w.botID, string(data))

	err := w.conn.WriteJSON(v)
	if err == nil {
		w.lastSendTime = time.Now()
	}
	return err
}

func (w *WeComClient) Close() {
	log.Printf("[WeCom] 关闭机器人 %s", w.botID)
	w.closeOnce.Do(func() {
		log.Printf("[WeCom] 执行关闭机器人 %s: 先断开 WebSocket", w.botID)
		w.connMutex.Lock()
		if w.conn != nil {
			w.conn.Close()
		}
		w.connMutex.Unlock()

		log.Printf("[WeCom] 关闭 stopChan (botID=%s)", w.botID)
		close(w.stopChan)
	})

	log.Printf("[WeCom] 等待 readLoop 退出 (botID=%s)", w.botID)
	w.wg.Wait()

	w.connMutex.Lock()
	w.connected = false
	w.connMutex.Unlock()
	w.notifyStatus("disconnected")

	log.Printf("[WeCom] 机器人 %s 已关闭", w.botID)
}

func (w *WeComClient) IsConnected() bool {
	w.connMutex.Lock()
	defer w.connMutex.Unlock()
	return w.connected
}

type BotInstance struct {
	ID             string
	Config         BotConfig
	WeCom          *WeComClient
	SkillEng       *SkillEngine
	SessionMgr     *SessionManager
	WeaviateClient *knowledge.WeaviateClient
}

type BotConfig struct {
	ID               string
	Name             string
	WecomBotID       string
	WecomSecret      string
	LLMProviderID    string
	LLMModel         string
	LLMTemperature   float64
	LLMMaxTokens     int
	MaxSessionTokens int
	Enabled          bool
	GoJudgeEndpoint  string
}

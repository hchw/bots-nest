package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type TaskProgress struct {
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	TotalChunks     int    `json:"total_chunks"`
	ProcessedChunks int    `json:"processed_chunks"`
}

type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (c *Client) Send(msg []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteMessage(websocket.TextMessage, msg)
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
	log.Printf("[WS Hub] 客户端已连接，当前连接数: %d", len(h.clients))
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.conn.Close()
		log.Printf("[WS Hub] 客户端已断开，当前连接数: %d", len(h.clients))
	}
}

func (h *Hub) BroadcastTaskProgress(taskID, status string, totalChunks, processedChunks int) {
	msg := TaskProgress{
		TaskID:          taskID,
		Status:          status,
		TotalChunks:     totalChunks,
		ProcessedChunks: processedChunks,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS Hub] 序列化消息失败: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := client.Send(data); err != nil {
			log.Printf("[WS Hub] 发送消息失败: %v", err)
			go h.Unregister(client)
		}
	}
}

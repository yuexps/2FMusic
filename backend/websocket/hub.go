package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"2fmusic/backend/config"
	"2fmusic/backend/logger"
	"2fmusic/backend/middleware"
	"2fmusic/backend/model"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

var GlobalHub = &Hub{
	broadcast:  make(chan []byte, 256),
	register:   make(chan *Client),
	unregister: make(chan *Client),
	clients:    make(map[*Client]bool),
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.Debug("WebSocket 客户端已建立连接")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				logger.Debug("WebSocket 客户端已断开")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastJSON 广播 JSON 结构
func BroadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err == nil {
		GlobalHub.broadcast <- data
	}
}

// NotifyLibraryChanged 广播曲库发生变动
func NotifyLibraryChanged() {
	BroadcastJSON(map[string]interface{}{
		"type":   "broadcast",
		"action": "library_changed",
		"data":   map[string]interface{}{"status": "updated"},
	})
}

// ServeWS 处理 HTTP 升级为 WebSocket
func ServeWS(c *gin.Context) {
	if config.GlobalConfig.Password != "" {
		pass := c.Query("auth")
		if pass == "" {
			pass = c.GetHeader("X-Password")
		}
		if !middleware.ValidatePassword(pass) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error("WS 升级失败: %v", err)
		return
	}

	client := &Client{hub: GlobalHub, conn: conn, send: make(chan []byte, 256)}
	GlobalHub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(10 * 1024 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var req model.WSClientRequest
		if err := json.Unmarshal(message, &req); err != nil {
			continue
		}

		if req.Action == "ping" {
			c.sendJSON(map[string]interface{}{
				"type": "pong",
			})
			continue
		}

		// 分发 WS Action Handler
		go HandleWSAction(c, req)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err == nil {
		c.send <- data
	}
}

func SendSuccessResponse(c *Client, seq interface{}, action string, data interface{}) {
	c.sendJSON(model.WSResponseFrame{
		Seq:     seq,
		Type:    "response",
		Action:  action,
		Success: true,
		Data:    data,
	})
}

func SendErrorResponse(c *Client, seq interface{}, action string, errMsg string) {
	c.sendJSON(model.WSResponseFrame{
		Seq:     seq,
		Type:    "response",
		Action:  action,
		Success: false,
		Error:   errMsg,
	})
}

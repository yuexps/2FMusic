package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"2fmusic/backend/core"
	"2fmusic/backend/db"
	"2fmusic/backend/static"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	mu     sync.Mutex
	closed bool
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
	register:   make(chan *Client, 64),
	unregister: make(chan *Client, 64),
	clients:    make(map[*Client]bool),
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			core.Debug("WebSocket 客户端已建立连接")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.safeClose()
				core.Debug("WebSocket 客户端已断开")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				if !client.safeSend(message) {
					client.safeClose()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastJSON 广播 JSON 结构 (非阻塞防卡死)
func BroadcastJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err == nil {
		select {
		case GlobalHub.broadcast <- data:
		default:
			core.Warn("WebSocket 广播通道满载，自动弃帧防止阻塞业务协程")
		}
	}
}

var (
	debounceMu       sync.Mutex
	debounceTimer    *time.Timer
	pendingSongIDs   = make(map[string]bool)
	pendingFields    = make(map[string]bool)
	pendingEventType = "update"
)

// NotifySongChangedDebounced 推送 200ms 防抖精细化歌曲变更广播
func NotifySongChangedDebounced(songID string, eventType string, fields []string) {
	if songID == "" {
		return
	}
	debounceMu.Lock()
	defer debounceMu.Unlock()

	pendingSongIDs[songID] = true
	for _, f := range fields {
		pendingFields[f] = true
	}
	if eventType != "" {
		pendingEventType = eventType
	}

	if debounceTimer != nil {
		debounceTimer.Stop()
	}

	debounceTimer = time.AfterFunc(200*time.Millisecond, flushDebouncedNotifications)
}

func flushDebouncedNotifications() {
	debounceMu.Lock()
	if len(pendingSongIDs) == 0 {
		debounceMu.Unlock()
		return
	}

	songIDs := make([]string, 0, len(pendingSongIDs))
	for id := range pendingSongIDs {
		songIDs = append(songIDs, id)
	}

	fields := make([]string, 0, len(pendingFields))
	for f := range pendingFields {
		fields = append(fields, f)
	}

	eventType := pendingEventType

	pendingSongIDs = make(map[string]bool)
	pendingFields = make(map[string]bool)
	pendingEventType = "update"
	debounceTimer = nil
	debounceMu.Unlock()

	if len(songIDs) > 50 {
		NotifyLibraryReloadAll()
		return
	}

	NotifyLibraryChangedDetailed(songIDs, eventType, fields)
}

// NotifyLibraryChangedDetailed 发送精细化广播
func NotifyLibraryChangedDetailed(songIDs []string, eventType string, fields []string) {
	if songIDs == nil {
		songIDs = []string{}
	}
	if fields == nil {
		fields = []string{}
	}
	BroadcastJSON(map[string]interface{}{
		"type":   "broadcast",
		"action": "library_changed",
		"data": map[string]interface{}{
			"event_type": eventType,
			"song_ids":   songIDs,
			"fields":     fields,
			"timestamp":  time.Now().UnixMilli(),
		},
	})
}

// NotifyLibraryReloadAll 广播全库全量刷新通知
func NotifyLibraryReloadAll() {
	NotifyLibraryChangedDetailed([]string{}, "reload_all", []string{})
}

// NotifyLibraryChanged 广播曲库全量变动兜底
func NotifyLibraryChanged() {
	NotifyLibraryReloadAll()
}

// BroadcastScanStatus 广播扫描与在线刮削实时进度数据
func BroadcastScanStatus(scanning, isScraping bool, total, processed, failed int, currentFile, currentPath string) {
	musicCnt, _ := db.GetSongCount()
	plCnt, _ := db.GetFavoritePlaylistCount()

	BroadcastJSON(map[string]interface{}{
		"type":   "broadcast",
		"action": "scan_status",
		"data": map[string]interface{}{
			"scanning":        scanning,
			"is_scraping":     isScraping,
			"total":           total,
			"processed":       processed,
			"failed":          failed,
			"current_file":    currentFile,
			"current_path":    currentPath,
			"music_count":     musicCnt,
			"playlist_count":  plCnt,
			"library_version": float64(time.Now().UnixNano()) / 1e9,
		},
	})
}

// ServeWS 处理 HTTP 升级为 WebSocket
func ServeWS(c *gin.Context) {
	if core.GlobalConfig.Password != "" {
		pass := c.Query("auth")
		if pass == "" {
			pass = c.GetHeader("X-Password")
		}
		if !static.ValidatePassword(pass) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		core.Error("WS 升级失败: %v", err)
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

	c.conn.SetReadLimit(1024 * 1024)
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

		var req core.WSClientRequest
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

func (c *Client) safeClose() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.send)
	}
}

func (c *Client) safeSend(data []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

func (c *Client) sendJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err == nil {
		c.safeSend(data)
	}
}

func SendSuccessResponse(c *Client, seq interface{}, action string, data interface{}) {
	c.sendJSON(core.WSResponseFrame{
		Seq:     seq,
		Type:    "response",
		Action:  action,
		Success: true,
		Data:    data,
	})
}

func SendErrorResponse(c *Client, seq interface{}, action string, errMsg string) {
	c.sendJSON(core.WSResponseFrame{
		Seq:     seq,
		Type:    "response",
		Action:  action,
		Success: false,
		Error:   errMsg,
	})
}

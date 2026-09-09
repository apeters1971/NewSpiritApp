package hub

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Envelope struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type Conn struct {
	conn      *websocket.Conn
	userID    string
	role      string
	send      chan []byte
	closeOnce sync.Once
}

func (c *Conn) closeSend() {
	c.closeOnce.Do(func() { close(c.send) })
}

type Hub struct {
	mu          sync.RWMutex
	members     map[string]map[*Conn]struct{}
	controllers map[*Conn]struct{}
}

func New() *Hub {
	return &Hub{
		members:     map[string]map[*Conn]struct{}{},
		controllers: map[*Conn]struct{}{},
	}
}

func (h *Hub) RegisterMember(conn *websocket.Conn, userID, role string) *Conn {
	c := &Conn{conn: conn, userID: userID, role: role, send: make(chan []byte, 256)}
	h.mu.Lock()
	if h.members[userID] == nil {
		h.members[userID] = map[*Conn]struct{}{}
	}
	h.members[userID][c] = struct{}{}
	h.mu.Unlock()
	go h.writer(c)
	return c
}

func (h *Hub) UnregisterMember(c *Conn) {
	h.mu.Lock()
	if set, ok := h.members[c.userID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.members, c.userID)
		}
	}
	h.mu.Unlock()
	c.closeSend()
}

func (h *Hub) RegisterController(conn *websocket.Conn) *Conn {
	c := &Conn{conn: conn, send: make(chan []byte, 256)}
	h.mu.Lock()
	h.controllers[c] = struct{}{}
	h.mu.Unlock()
	go h.writer(c)
	return c
}

func (h *Hub) UnregisterController(c *Conn) {
	h.mu.Lock()
	delete(h.controllers, c)
	h.mu.Unlock()
	c.closeSend()
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.members)
}

func (h *Hub) Broadcast(env Envelope) {
	raw, err := json.Marshal(env)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, set := range h.members {
		for c := range set {
			select {
			case c.send <- raw:
			default:
			}
		}
	}
	for c := range h.controllers {
		select {
		case c.send <- raw:
		default:
		}
	}
}

func (h *Hub) writer(c *Conn) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				_ = c.conn.Close()
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				_ = c.conn.Close()
				return
			}
		}
	}
}

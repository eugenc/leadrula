package messaging

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// typingTTL is how long a typing signal is considered active before expiring.
const typingTTL = 5 * time.Second

// WSEvent is a server→client or client→server envelope.
type WSEvent struct {
	Type     string          `json:"type"`
	ThreadID string          `json:"thread_id,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// client is one authenticated WebSocket connection.
type client struct {
	conn      *websocket.Conn
	accountID int64
	userID    int64
	userName  string
	send      chan []byte
}

type typer struct {
	userID   int64
	userName string
	expires  time.Time
}

// Hub fans out messaging events to connected clients and tracks ephemeral
// typing state per thread (no DB persistence).
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	// typing[threadPublicID][userID] = typer
	typing map[string]map[int64]typer
}

func NewHub() *Hub {
	h := &Hub{
		clients: map[*client]struct{}{},
		typing:  map[string]map[int64]typer{},
	}
	go h.reapTyping()
	return h
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// broadcastToAccounts sends an event to every client on any of the accounts.
func (h *Hub) broadcastToAccounts(accountIDs []int64, evt WSEvent) {
	if len(accountIDs) == 0 {
		return
	}
	set := make(map[int64]struct{}, len(accountIDs))
	for _, id := range accountIDs {
		set[id] = struct{}{}
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if _, ok := set[c.accountID]; !ok {
			continue
		}
		select {
		case c.send <- data:
		default:
		}
	}
}

// broadcastToUsers sends an event to specific users (used for internal threads).
func (h *Hub) broadcastToUsers(userIDs []int64, evt WSEvent) {
	if len(userIDs) == 0 {
		return
	}
	set := make(map[int64]struct{}, len(userIDs))
	for _, id := range userIDs {
		set[id] = struct{}{}
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if _, ok := set[c.userID]; !ok {
			continue
		}
		select {
		case c.send <- data:
		default:
		}
	}
}

// setTyping records a typing signal and returns the payload to fan out.
func (h *Hub) setTyping(threadPublicID string, userID int64, userName string) {
	h.mu.Lock()
	m := h.typing[threadPublicID]
	if m == nil {
		m = map[int64]typer{}
		h.typing[threadPublicID] = m
	}
	m[userID] = typer{userID: userID, userName: userName, expires: time.Now().Add(typingTTL)}
	h.mu.Unlock()
}

func (h *Hub) clearTyping(threadPublicID string, userID int64) {
	h.mu.Lock()
	if m := h.typing[threadPublicID]; m != nil {
		delete(m, userID)
	}
	h.mu.Unlock()
}

// reapTyping periodically drops expired typing entries and notifies clients.
func (h *Hub) reapTyping() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		type stopped struct {
			thread string
			userID int64
		}
		var expired []stopped
		h.mu.Lock()
		for thread, m := range h.typing {
			for uid, t := range m {
				if now.After(t.expires) {
					delete(m, uid)
					expired = append(expired, stopped{thread, uid})
				}
			}
			if len(m) == 0 {
				delete(h.typing, thread)
			}
		}
		h.mu.Unlock()
		for _, e := range expired {
			payload, _ := json.Marshal(map[string]any{"thread_id": e.thread, "user_id": e.userID})
			h.broadcastAll(WSEvent{Type: "user_stopped_typing", ThreadID: e.thread, Payload: payload})
		}
	}
}

// broadcastAll sends to every client (typing stop cleanup); receivers filter
// by whether they have the thread open.
func (h *Hub) broadcastAll(evt WSEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
)

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump handles inbound client events (subscribe/read/typing).
func (c *client) readPump(h *Hub, onTyping func(threadPublicID string, typing bool)) {
	defer func() {
		h.remove(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var evt WSEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			continue
		}
		switch evt.Type {
		case "typing":
			if evt.ThreadID != "" {
				onTyping(evt.ThreadID, true)
			}
		case "stop_typing":
			if evt.ThreadID != "" {
				onTyping(evt.ThreadID, false)
			}
		case "subscribe", "read":
			// no-op: subscription is implicit; read receipts go through REST
		default:
			log.Printf("messaging ws: unknown client event %q", evt.Type)
		}
	}
}

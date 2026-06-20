package service

import (
	"fmt"
	"log/slog"
	"sync"

	"innovation-incubation-platform-backend/internal/model"
)

const maxSSEConnsPerUser = 10

type SSEEvent struct {
	ID         uint                   `json:"id"`
	CreatedAt  int64                  `json:"created_at"`
	Type       model.NotificationType `json:"type"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	TargetType model.TargetType       `json:"target_type,omitempty"`
	TargetID   uint                   `json:"target_id,omitempty"`
}

type SSEHub struct {
	mu      sync.RWMutex
	clients map[uint]map[chan SSEEvent]struct{}
}

func NewSSEHub() *SSEHub {
	return &SSEHub{clients: make(map[uint]map[chan SSEEvent]struct{})}
}

func (h *SSEHub) Subscribe(userID uint) (chan SSEEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.clients[userID]) >= maxSSEConnsPerUser {
		return nil, fmt.Errorf("too many connections")
	}

	ch := make(chan SSEEvent, 16)
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan SSEEvent]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	return ch, nil
}

func (h *SSEHub) Unsubscribe(userID uint, ch chan SSEEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] != nil {
		if _, ok := h.clients[userID][ch]; ok {
			delete(h.clients[userID], ch)
			close(ch)
		}
		if len(h.clients[userID]) == 0 {
			delete(h.clients, userID)
		}
	}
}

func (h *SSEHub) Notify(userID uint, event SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.clients[userID] == nil {
		return
	}
	for ch := range h.clients[userID] {
		select {
		case ch <- event:
		default:
			slog.Warn("SSE notify dropped", "user_id", userID, "event_type", event.Type)
		}
	}
}

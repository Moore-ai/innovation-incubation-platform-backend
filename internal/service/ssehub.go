package service

import (
	"sync"

	"innovation-incubation-platform-backend/internal/model"
)

type SSEEvent struct {
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

func (h *SSEHub) Subscribe(userID uint) chan SSEEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan SSEEvent, 16)
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan SSEEvent]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	return ch
}

func (h *SSEHub) Unsubscribe(userID uint, ch chan SSEEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] != nil {
		delete(h.clients[userID], ch)
		close(ch)
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
		}
	}
}

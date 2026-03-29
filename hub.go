package main

import (
	"sync"

	"github.com/ismail/tsnake/game"
)

type Hub struct {
	mu       sync.RWMutex
	channels map[string]chan game.GameSnapshot
}

func NewHub() *Hub {
	return &Hub{
		channels: make(map[string]chan game.GameSnapshot),
	}
}

func (h *Hub) Register(id string, initial game.GameSnapshot) <-chan game.GameSnapshot {
	ch := make(chan game.GameSnapshot, 1)
	sendLatest(ch, initial)

	h.mu.Lock()
	h.channels[id] = ch
	h.mu.Unlock()

	return ch
}

func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	delete(h.channels, id)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(snap game.GameSnapshot) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, ch := range h.channels {
		sendLatest(ch, snap)
	}
}

func sendLatest(ch chan game.GameSnapshot, snap game.GameSnapshot) {
	select {
	case ch <- snap:
		return
	default:
	}

	select {
	case <-ch:
	default:
	}

	select {
	case ch <- snap:
	default:
	}
}

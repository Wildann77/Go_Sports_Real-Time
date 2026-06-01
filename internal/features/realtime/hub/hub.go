package hub

import (
	"context"
	"log/slog"
	"sync"
)

// Hub maintains the set of active clients and broadcasts messages to the rooms.
type Hub struct {
	rooms       map[int64]*Room
	clients     map[*Client]bool
	Broadcast   chan *BroadcastMessage
	Register    chan *Client
	Unregister  chan *Client
	Subscribe   chan *SubscriptionRequest
	Unsubscribe chan *SubscriptionRequest
	stop        chan struct{}
	done        chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
}

func NewHub() *Hub {
	return &Hub{
		rooms:       make(map[int64]*Room),
		clients:     make(map[*Client]bool),
		Broadcast:   make(chan *BroadcastMessage, 64),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		Subscribe:   make(chan *SubscriptionRequest),
		Unsubscribe: make(chan *SubscriptionRequest),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

func (h *Hub) Start(ctx context.Context) {
	h.startOnce.Do(func() {
		go h.Run(ctx)
	})
}

func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stop)
	})
}

func (h *Hub) StopCh() <-chan struct{} {
	return h.stop
}

func (h *Hub) Run(ctx context.Context) {
	defer close(h.done)

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return

		case <-h.stop:
			h.shutdown()
			return

		case client := <-h.Register:
			h.clients[client] = true
			slog.Info("Client registered", "client", client.id)

		case client := <-h.Unregister:
			delete(h.clients, client)
			slog.Info("Client unregistered", "client", client.id)
			h.cleanupClientInfo(client)
			client.Close()

		case subReq := <-h.Subscribe:
			h.handleSubscribe(subReq)

		case unsubReq := <-h.Unsubscribe:
			h.handleUnsubscribe(unsubReq)

		case payload := <-h.Broadcast:
			room, ok := h.rooms[payload.RoomID]
			if ok {
				for client := range room.clients {
					select {
					case client.send <- payload.Message:
					default:
						h.removeClientFromRoom(client, payload.RoomID)
					}
				}
			}
		}
	}
}

func (h *Hub) handleSubscribe(req *SubscriptionRequest) {
	room, ok := h.rooms[req.RoomID]
	if !ok {
		room = &Room{
			id:      req.RoomID,
			clients: make(map[*Client]bool),
		}
		h.rooms[req.RoomID] = room
	}
	room.clients[req.Client] = true
	req.Client.rooms[req.RoomID] = true
	slog.Info("Client subscribed to room", "client", req.Client.id, "room", req.RoomID)
}

func (h *Hub) handleUnsubscribe(req *SubscriptionRequest) {
	h.removeClientFromRoom(req.Client, req.RoomID)
	slog.Info("Client unsubscribed from room", "client", req.Client.id, "room", req.RoomID)
}

func (h *Hub) cleanupClientInfo(client *Client) {
	for roomID := range client.rooms {
		h.removeClientFromRoom(client, roomID)
	}
}

func (h *Hub) removeClientFromRoom(client *Client, roomID int64) {
	if room, ok := h.rooms[roomID]; ok {
		delete(room.clients, client)
		if len(room.clients) == 0 {
			delete(h.rooms, roomID) // Cleanup empty room
		}
	}
	delete(client.rooms, roomID)
}

func (h *Hub) BroadcastToRoom(roomID int64, eventType string, data any) {
	msg := NewWebSocketMessage(eventType, roomID, data)
	payload := &BroadcastMessage{
		RoomID:  roomID,
		Message: msg,
	}
	select {
	case <-h.stop:
		return
	case h.Broadcast <- payload:
	}
}

func (h *Hub) RegisterClient(client *Client) bool {
	select {
	case <-h.stop:
		return false
	case h.Register <- client:
		return true
	}
}

func (h *Hub) UnregisterClient(client *Client) {
	select {
	case <-h.stop:
		return
	case h.Unregister <- client:
	}
}

func (h *Hub) SubscribeClient(req *SubscriptionRequest) bool {
	select {
	case <-h.stop:
		return false
	case h.Subscribe <- req:
		return true
	}
}

func (h *Hub) UnsubscribeClient(req *SubscriptionRequest) bool {
	select {
	case <-h.stop:
		return false
	case h.Unsubscribe <- req:
		return true
	}
}

func (h *Hub) shutdown() {
	for client := range h.clients {
		client.Close()
		delete(h.clients, client)
	}
	h.rooms = make(map[int64]*Room)
}

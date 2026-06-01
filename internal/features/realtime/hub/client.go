package hub

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"sports-dashboard/internal/shared/enums"
	"sports-dashboard/internal/shared/helpers"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1048576 // 1MB default
)

type Client struct {
	id         string
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	rooms      map[int64]bool // Set of rooms subscribed to
	closeOnce  sync.Once
	maxMsgSize int64
}

func NewClient(hub *Hub, conn *websocket.Conn, maxMsgSize int64) *Client {
	if maxMsgSize <= 0 {
		maxMsgSize = maxMessageSize
	}
	return &Client{
		id:         uuid.New().String(),
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, 256),
		rooms:      make(map[int64]bool),
		maxMsgSize: maxMsgSize,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.Close()
	}()
	c.conn.SetReadLimit(c.maxMsgSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket unexpected close", "error", err)
			}
			break
		}

		// Reset deadline on ANY incoming message because client uses JSON-based {"type":"ping"} instead of control frames
		c.conn.SetReadDeadline(time.Now().Add(pongWait))

		c.processMessage(message)
	}
}

func (c *Client) processMessage(message []byte) {
	if !helpers.IsValidJSONBytes(message) {
		c.sendError("invalid json message structure")
		return
	}

	var parsedMsg map[string]interface{}
	_ = json.Unmarshal(message, &parsedMsg)

	msgType, ok := parsedMsg["type"].(string)
	if !ok {
		c.sendError("missing or invalid 'type' field")
		return
	}

	if !enums.WSEventType(msgType).IsValid() {
		c.sendError("unknown or invalid message type")
		return
	}

	switch msgType {
	case string(enums.WSEventPing):
		c.sendEvent(string(enums.WSEventPong), 0, nil)

	case string(enums.WSEventSubscribe):
		matchID, ok := parsedMsg["matchId"].(float64)
		if !ok || matchID <= 0 {
			c.sendError("invalid or missing matchId")
			return
		}
		// Optional logic could be added here to check if the match actually exists in DB
		if !c.hub.SubscribeClient(&SubscriptionRequest{Client: c, RoomID: int64(matchID)}) {
			return
		}
		c.sendEvent(string(enums.WSEventSubscribed), int64(matchID), nil)

	case string(enums.WSEventUnsubscribe):
		matchID, ok := parsedMsg["matchId"].(float64)
		if !ok || matchID <= 0 {
			c.sendError("invalid or missing matchId")
			return
		}
		if !c.hub.UnsubscribeClient(&SubscriptionRequest{Client: c, RoomID: int64(matchID)}) {
			return
		}
		c.sendEvent(string(enums.WSEventUnsubscribed), int64(matchID), nil)

	default:
		c.sendError("unknown message type")
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
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

func (c *Client) SendDirect(message []byte) {
	select {
	case <-c.hub.StopCh():
		return
	default:
	}

	select {
	case c.send <- message:
	default:
	}
}

func (c *Client) sendEvent(eventType string, matchID int64, data any) {
	msg := NewWebSocketMessage(eventType, matchID, data)
	select {
	case <-c.hub.StopCh():
		return
	default:
	}

	select {
	case c.send <- msg:
	default:
		slog.Warn("Failed to send message, channel full", "client", c.id)
	}
}

func (c *Client) sendError(errMsg string) {
	msg := map[string]string{
		"type":    string(enums.WSEventError),
		"message": errMsg,
	}
	b, _ := json.Marshal(msg)
	select {
	case <-c.hub.StopCh():
		return
	default:
	}

	select {
	case c.send <- b:
	default:
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		if c.conn != nil {
			_ = c.conn.Close()
		}
		close(c.send)
	})
}

func (c *Client) ID() string {
	return c.id
}

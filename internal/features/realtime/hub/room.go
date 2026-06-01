package hub

import (
	"encoding/json"
)

type Room struct {
	id      int64
	clients map[*Client]bool
}

type SubscriptionRequest struct {
	Client *Client
	RoomID int64
}

type BroadcastMessage struct {
	RoomID  int64
	Message []byte
}

func NewWebSocketMessage(eventType string, matchID int64, data any) []byte {
	msg := map[string]interface{}{
		"type": eventType,
	}
	if matchID > 0 {
		msg["matchId"] = matchID
	}
	if data != nil {
		msg["data"] = data
	}
	response, _ := json.Marshal(msg)
	return response
}

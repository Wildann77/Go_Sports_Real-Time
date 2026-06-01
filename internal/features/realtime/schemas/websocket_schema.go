package schemas

type WebSocketIncomingMessage struct {
	Type    string `json:"type"`
	MatchID int64  `json:"matchId,omitempty"`
}

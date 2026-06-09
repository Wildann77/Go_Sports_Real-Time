package enums

type WSEventType string

const (
	WSEventWelcome           WSEventType = "welcome"
	WSEventSubscribe         WSEventType = "subscribe"
	WSEventSubscribed        WSEventType = "subscribed"
	WSEventUnsubscribe       WSEventType = "unsubscribe"
	WSEventUnsubscribed      WSEventType = "unsubscribed"
	WSEventPing              WSEventType = "ping"
	WSEventPong              WSEventType = "pong"
	WSEventError             WSEventType = "error"
	WSEventCommentaryCreated WSEventType = "commentary.created"
	WSEventMatchUpdated      WSEventType = "match.updated"
)

func (s WSEventType) IsValid() bool {
	switch s {
	case WSEventWelcome, WSEventSubscribe, WSEventSubscribed, WSEventUnsubscribe, WSEventUnsubscribed, WSEventPing, WSEventPong, WSEventError, WSEventCommentaryCreated, WSEventMatchUpdated:
		return true
	}
	return false
}

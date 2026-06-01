package hub

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"sports-dashboard/internal/shared/enums"
)

func TestClientProcessMessageInvalidJSONRejected(t *testing.T) {
	client := newTestClient(nil)

	client.processMessage([]byte("{"))

	assertClientErrorMessage(t, client, "invalid json message structure")
}

func TestClientProcessMessageMissingTypeRejected(t *testing.T) {
	client := newTestClient(nil)

	client.processMessage([]byte(`{"matchId":1}`))

	assertClientErrorMessage(t, client, "missing or invalid 'type' field")
}

func TestClientProcessMessageInvalidTypeRejected(t *testing.T) {
	client := newTestClient(nil)

	client.processMessage([]byte(`{"type":"bogus"}`))

	assertClientErrorMessage(t, client, "unknown or invalid message type")
}

func TestClientProcessMessagePingReturnsPong(t *testing.T) {
	client := newTestClient(nil)

	client.processMessage([]byte(`{"type":"ping"}`))

	msg := assertClientMessage(t, client)
	if msg["type"] != string(enums.WSEventPong) {
		t.Fatalf("expected pong event, got %#v", msg["type"])
	}
}

func TestClientProcessMessageSubscribeAndUnsubscribeCommands(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)

	client := newTestClient(h)
	if !h.RegisterClient(client) {
		t.Fatal("expected registration to succeed")
	}

	client.processMessage([]byte(`{"type":"subscribe","matchId":7}`))

	msg := assertClientMessage(t, client)
	if msg["type"] != string(enums.WSEventSubscribed) {
		t.Fatalf("expected subscribed event, got %#v", msg["type"])
	}

	h.BroadcastToRoom(7, string(enums.WSEventCommentaryCreated), map[string]any{"message": "goal"})
	msg = assertClientMessage(t, client)
	if msg["type"] != string(enums.WSEventCommentaryCreated) {
		t.Fatalf("expected room broadcast after subscribe, got %#v", msg["type"])
	}

	client.processMessage([]byte(`{"type":"unsubscribe","matchId":7}`))

	msg = assertClientMessage(t, client)
	if msg["type"] != string(enums.WSEventUnsubscribed) {
		t.Fatalf("expected unsubscribed event, got %#v", msg["type"])
	}

	h.BroadcastToRoom(7, string(enums.WSEventCommentaryCreated), map[string]any{"message": "should-not-arrive"})
	assertNoClientMessage(t, client, 250*time.Millisecond)

	h.Stop()
	waitForHubShutdown(t, h)

	if _, roomExists := h.rooms[7]; roomExists {
		t.Fatal("expected room cleanup after unsubscribe")
	}
	if _, subscribed := client.rooms[7]; subscribed {
		t.Fatal("expected client room membership cleanup after unsubscribe")
	}
}

func newTestClient(h *Hub) *Client {
	if h == nil {
		h = NewHub()
	}

	return &Client{
		id:         "test-client",
		hub:        h,
		send:       make(chan []byte, 8),
		rooms:      make(map[int64]bool),
		maxMsgSize: 1024,
	}
}

func assertClientErrorMessage(t *testing.T, client *Client, expected string) {
	t.Helper()

	msg := assertClientMessage(t, client)
	if msg["type"] != string(enums.WSEventError) {
		t.Fatalf("expected error event, got %#v", msg["type"])
	}
	if msg["message"] != expected {
		t.Fatalf("expected error message %q, got %#v", expected, msg["message"])
	}
}

func assertClientMessage(t *testing.T, client *Client) map[string]any {
	t.Helper()

	select {
	case raw := <-client.send:
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("failed to decode client message: %v", err)
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client message")
		return nil
	}
}

func assertNoClientMessage(t *testing.T, client *Client, within time.Duration) {
	t.Helper()

	select {
	case raw := <-client.send:
		t.Fatalf("expected no client message, got %s", string(raw))
	case <-time.After(within):
	}
}

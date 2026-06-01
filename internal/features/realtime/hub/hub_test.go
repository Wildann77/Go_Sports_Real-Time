package hub

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"sports-dashboard/internal/shared/enums"
)

func TestHubBroadcastsToSubscribedClient(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)

	client := &Client{
		id:    "client-1",
		hub:   h,
		send:  make(chan []byte, 1),
		rooms: make(map[int64]bool),
	}

	if !h.RegisterClient(client) {
		t.Fatal("expected client registration to succeed")
	}

	if !h.SubscribeClient(&SubscriptionRequest{Client: client, RoomID: 7}) {
		t.Fatal("expected client subscription to succeed")
	}

	h.BroadcastToRoom(7, string(enums.WSEventCommentaryCreated), map[string]any{"message": "goal"})

	select {
	case msg := <-client.send:
		var payload map[string]any
		if err := json.Unmarshal(msg, &payload); err != nil {
			t.Fatalf("failed to decode broadcast payload: %v", err)
		}

		if payload["type"] != string(enums.WSEventCommentaryCreated) {
			t.Fatalf("expected event type %s, got %v", enums.WSEventCommentaryCreated, payload["type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for broadcast")
	}

	h.Stop()

	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hub shutdown")
	}
}

func TestHubOperationsAfterStopDoNotBlock(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)
	h.Stop()

	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hub shutdown")
	}

	client := &Client{
		id:    "client-2",
		hub:   h,
		send:  make(chan []byte, 1),
		rooms: make(map[int64]bool),
	}

	if h.RegisterClient(client) {
		t.Fatal("expected registration to fail after stop")
	}

	if h.SubscribeClient(&SubscriptionRequest{Client: client, RoomID: 9}) {
		t.Fatal("expected subscribe to fail after stop")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.BroadcastToRoom(9, string(enums.WSEventMatchUpdated), map[string]any{"status": "live"})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broadcast blocked after stop")
	}
}

func TestHubRegisterAndUnregisterBehavior(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)

	client := &Client{
		id:    "client-register",
		hub:   h,
		send:  make(chan []byte, 2),
		rooms: make(map[int64]bool),
	}

	if !h.RegisterClient(client) {
		t.Fatal("expected registration to succeed")
	}
	if !h.SubscribeClient(&SubscriptionRequest{Client: client, RoomID: 11}) {
		t.Fatal("expected subscribe to succeed")
	}

	h.BroadcastToRoom(11, string(enums.WSEventMatchUpdated), map[string]any{"status": "live"})
	select {
	case <-client.send:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for broadcast to registered client")
	}

	h.UnregisterClient(client)
	time.Sleep(50 * time.Millisecond)
	h.Stop()
	waitForHubShutdown(t, h)

	if _, ok := h.clients[client]; ok {
		t.Fatal("expected client removal after unregister")
	}
}

func TestHubSubscribeAndUnsubscribeBehavior(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)

	client := &Client{
		id:    "client-subscribe",
		hub:   h,
		send:  make(chan []byte, 2),
		rooms: make(map[int64]bool),
	}

	if !h.RegisterClient(client) {
		t.Fatal("expected registration to succeed")
	}
	if !h.SubscribeClient(&SubscriptionRequest{Client: client, RoomID: 22}) {
		t.Fatal("expected subscribe to succeed")
	}

	h.BroadcastToRoom(22, string(enums.WSEventCommentaryCreated), map[string]any{"message": "goal"})
	select {
	case <-client.send:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for room broadcast")
	}

	if !h.UnsubscribeClient(&SubscriptionRequest{Client: client, RoomID: 22}) {
		t.Fatal("expected unsubscribe to succeed")
	}

	h.BroadcastToRoom(22, string(enums.WSEventCommentaryCreated), map[string]any{"message": "should-not-arrive"})
	select {
	case msg := <-client.send:
		t.Fatalf("expected no message after unsubscribe, got %s", string(msg))
	case <-time.After(250 * time.Millisecond):
	}

	h.Stop()
	waitForHubShutdown(t, h)

	if _, roomExists := h.rooms[22]; roomExists {
		t.Fatal("expected empty room cleanup after unsubscribe")
	}
	if _, subscribed := client.rooms[22]; subscribed {
		t.Fatal("expected client room membership cleanup after unsubscribe")
	}
}

func TestHubBroadcastRemovesSlowClientAndCleansEmptyRoom(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h.Start(ctx)

	slowClient := &Client{
		id:    "slow-client",
		hub:   h,
		send:  make(chan []byte, 1),
		rooms: make(map[int64]bool),
	}
	slowClient.send <- []byte("already-full")

	if !h.RegisterClient(slowClient) {
		t.Fatal("expected registration to succeed")
	}
	if !h.SubscribeClient(&SubscriptionRequest{Client: slowClient, RoomID: 99}) {
		t.Fatal("expected subscribe to succeed")
	}

	h.BroadcastToRoom(99, string(enums.WSEventMatchUpdated), map[string]any{"status": "live"})
	time.Sleep(50 * time.Millisecond)
	h.Stop()
	waitForHubShutdown(t, h)

	if _, roomExists := h.rooms[99]; roomExists {
		t.Fatal("expected slow client room cleanup after removal")
	}
	if _, subscribed := slowClient.rooms[99]; subscribed {
		t.Fatal("expected slow client membership cleanup after removal")
	}
}

func assertEventually(t *testing.T, check func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", description)
}

func waitForHubShutdown(t *testing.T, h *Hub) {
	t.Helper()

	select {
	case <-h.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for hub shutdown")
	}
}

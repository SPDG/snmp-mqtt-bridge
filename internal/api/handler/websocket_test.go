package handler

import (
	"encoding/json"
	"testing"

	"github.com/gorilla/websocket"
)

func newTestWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		clients:   make(map[*wsClient]bool),
		broadcast: make(chan []byte, 1),
	}
}

func TestWebSocketHandlerUnregisterClosesClientSendOnce(t *testing.T) {
	h := newTestWebSocketHandler()
	client := &wsClient{send: make(chan wsMessage, 1)}

	h.registerClient(client)
	h.unregisterClient(client)
	h.unregisterClient(client)

	if _, ok := h.clients[client]; ok {
		t.Fatal("client still registered after unregister")
	}

	if _, ok := <-client.send; ok {
		t.Fatal("client send channel is still open after unregister")
	}
}

func TestWebSocketHandlerBroadcastRemovesFullClient(t *testing.T) {
	h := newTestWebSocketHandler()
	fullClient := &wsClient{send: make(chan wsMessage, 1)}
	readyClient := &wsClient{send: make(chan wsMessage, 1)}

	fullClient.send <- wsMessage{messageType: websocket.TextMessage, data: []byte("queued")}
	h.registerClient(fullClient)
	h.registerClient(readyClient)

	h.broadcastToClients([]byte("update"))

	if _, ok := h.clients[fullClient]; ok {
		t.Fatal("full client still registered after failed enqueue")
	}
	if _, ok := h.clients[readyClient]; !ok {
		t.Fatal("ready client was unexpectedly unregistered")
	}

	message := <-readyClient.send
	if message.messageType != websocket.TextMessage || string(message.data) != "update" {
		t.Fatalf("ready client received %+v, want text update", message)
	}

	if _, ok := <-fullClient.send; !ok {
		t.Fatal("expected queued message before closed channel")
	}
	if _, ok := <-fullClient.send; ok {
		t.Fatal("full client send channel is still open")
	}
}

func TestWebSocketHandlerPingQueuesPong(t *testing.T) {
	h := newTestWebSocketHandler()
	client := &wsClient{send: make(chan wsMessage, 1)}
	h.registerClient(client)

	h.handleMessage(client, []byte(`{"type":"ping"}`))

	message := <-client.send
	if message.messageType != websocket.TextMessage {
		t.Fatalf("message type = %d, want %d", message.messageType, websocket.TextMessage)
	}

	var payload map[string]string
	if err := json.Unmarshal(message.data, &payload); err != nil {
		t.Fatalf("failed to unmarshal pong payload: %v", err)
	}
	if payload["type"] != "pong" {
		t.Fatalf("payload type = %q, want pong", payload["type"])
	}
}

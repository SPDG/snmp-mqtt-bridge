package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"snmp-mqtt-bridge/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	wsWriteWait      = 10 * time.Second
	wsPongWait       = 60 * time.Second
	wsPingPeriod     = 30 * time.Second
	wsMaxMessageSize = 512
	wsSendBufferSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

type wsMessage struct {
	messageType int
	data        []byte
}

type wsClient struct {
	conn *websocket.Conn
	send chan wsMessage
}

// WebSocketHandler handles WebSocket connections for real-time updates
type WebSocketHandler struct {
	pollerService *service.PollerService
	clients       map[*wsClient]bool
	mu            sync.RWMutex
	broadcast     chan []byte
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(pollerService *service.PollerService) *WebSocketHandler {
	h := &WebSocketHandler{
		pollerService: pollerService,
		clients:       make(map[*wsClient]bool),
		broadcast:     make(chan []byte, 256),
	}

	// Start broadcast handler
	go h.handleBroadcasts()

	// Subscribe to poller events if available
	if pollerService != nil {
		go h.subscribeToPollerEvents()
	}

	return h
}

// HandleWebSocket upgrades HTTP connection to WebSocket
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan wsMessage, wsSendBufferSize),
	}

	h.registerClient(client)
	go h.writePump(client)

	// Send initial state
	h.sendInitialState(client)

	// Handle incoming messages
	h.readPump(client)
}

func (h *WebSocketHandler) registerClient(client *wsClient) {
	h.mu.Lock()
	h.clients[client] = true
	h.mu.Unlock()
}

func (h *WebSocketHandler) unregisterClient(client *wsClient) {
	var conn *websocket.Conn

	h.mu.Lock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
		conn = client.conn
	}
	h.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
}

func (h *WebSocketHandler) enqueue(client *wsClient, message wsMessage) bool {
	h.mu.RLock()
	_, ok := h.clients[client]
	if !ok {
		h.mu.RUnlock()
		return false
	}

	select {
	case client.send <- message:
		h.mu.RUnlock()
		return true
	default:
		h.mu.RUnlock()
		h.unregisterClient(client)
		return false
	}
}

func (h *WebSocketHandler) readPump(client *wsClient) {
	defer func() {
		h.unregisterClient(client)
		client.conn.Close()
	}()

	client.conn.SetReadLimit(wsMaxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages (e.g., subscribe to specific devices)
		h.handleMessage(client, message)
	}
}

func (h *WebSocketHandler) writePump(client *wsClient) {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		h.unregisterClient(client)
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}

			if err := client.conn.WriteMessage(message.messageType, message.data); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *WebSocketHandler) handleMessage(client *wsClient, message []byte) {
	var msg struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "ping":
		response := map[string]string{"type": "pong"}
		data, err := json.Marshal(response)
		if err != nil {
			return
		}
		h.enqueue(client, wsMessage{messageType: websocket.TextMessage, data: data})
	case "subscribe":
		// Handle subscription to specific device updates
	}
}

func (h *WebSocketHandler) sendInitialState(client *wsClient) {
	if h.pollerService == nil {
		return
	}

	states := h.pollerService.GetAllDeviceStates()
	msg := map[string]interface{}{
		"type": "initial_state",
		"data": states,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.enqueue(client, wsMessage{messageType: websocket.TextMessage, data: data})
}

func (h *WebSocketHandler) handleBroadcasts() {
	for message := range h.broadcast {
		h.broadcastToClients(message)
	}
}

func (h *WebSocketHandler) broadcastToClients(message []byte) {
	h.mu.RLock()
	clients := make([]*wsClient, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	for _, client := range clients {
		h.enqueue(client, wsMessage{messageType: websocket.TextMessage, data: message})
	}
}

func (h *WebSocketHandler) subscribeToPollerEvents() {
	if h.pollerService == nil {
		return
	}

	eventChan := h.pollerService.Subscribe()

	for event := range eventChan {
		msg := map[string]interface{}{
			"type": "state_update",
			"data": event,
		}

		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}

		select {
		case h.broadcast <- data:
		default:
			// Channel full, skip this message
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *WebSocketHandler) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		// Channel full, skip
	}
}

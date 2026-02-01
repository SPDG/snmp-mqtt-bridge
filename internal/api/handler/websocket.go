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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// WebSocketHandler handles WebSocket connections for real-time updates
type WebSocketHandler struct {
	pollerService *service.PollerService
	clients       map[*websocket.Conn]bool
	mu            sync.RWMutex
	broadcast     chan []byte
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(pollerService *service.PollerService) *WebSocketHandler {
	h := &WebSocketHandler{
		pollerService: pollerService,
		clients:       make(map[*websocket.Conn]bool),
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

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	// Send initial state
	h.sendInitialState(conn)

	// Handle incoming messages
	go h.handleClient(conn)
}

func (h *WebSocketHandler) handleClient(conn *websocket.Conn) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages (e.g., subscribe to specific devices)
		h.handleMessage(conn, message)
	}
}

func (h *WebSocketHandler) handleMessage(conn *websocket.Conn, message []byte) {
	var msg struct {
		Type string `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "ping":
		response := map[string]string{"type": "pong"}
		data, _ := json.Marshal(response)
		conn.WriteMessage(websocket.TextMessage, data)
	case "subscribe":
		// Handle subscription to specific device updates
	}
}

func (h *WebSocketHandler) sendInitialState(conn *websocket.Conn) {
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

	conn.WriteMessage(websocket.TextMessage, data)
}

func (h *WebSocketHandler) handleBroadcasts() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			// Send ping to all clients
			h.mu.RLock()
			for client := range h.clients {
				if err := client.WriteMessage(websocket.PingMessage, nil); err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
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

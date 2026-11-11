// Maintain connected clients, register/unregister clients, and broadcast messages.
package websocket

import (
	"encoding/json"
)

type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	clients    map[*Client]bool
}

type Event struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		// Register incoming client.
		case client := <-h.register:
			h.clients[client] = true

		// Unregister and close disconnected client.
		case client := <-h.unregister:
			if _, exists := h.clients[client]; exists {
				delete(h.clients, client)
				close(client.send)
			}

		// Broadcast JSON events to connected clients.
		case event := <-h.broadcast:
			message, err := json.Marshal(event)
			if err != nil {
				continue
			}

			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Remove clients that cannot keep up.
					delete(h.clients, client)
					close(client.send)
					client.conn.Close()
				}
			}
		}
	}
}

// Broadcast an event to all connected clients
func (h *Hub) Broadcast(event Event) {
	h.broadcast <- event
}

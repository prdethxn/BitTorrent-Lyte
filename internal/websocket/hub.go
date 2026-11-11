// Maintain connected clients, register/unregister clients, and broadcast messages
package websocket

type Hub struct {
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	clients    map[*Client]bool
}

func NewHub() *Hub {

	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte),
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
		// Broadcast message to connected clients.
		case message := <-h.broadcast:
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

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- append([]byte(nil), message...)
}

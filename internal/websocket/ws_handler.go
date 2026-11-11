// Configure WebSocket upgrader and handle incoming upgrade requests.
package websocket

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Configure WebSocket upgrader.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "http://localhost:5173"
	},
}

func (h *Hub) ServeWs(w http.ResponseWriter, r *http.Request) {
	// Upgrade incoming HTTP connection to WebSocket.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		return
	}

	// Register client and initialise buffered message channel.
	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	// Start independent read and write goroutines.
	go client.ReadPump()
	go client.WritePump()
}

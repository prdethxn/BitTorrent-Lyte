// Management websocket connections & read/write loops
package websocket

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte //buffered outgoing message queue
}

func (c *Client) ReadPump() {
	//unregister client and clean up connection after loop break
	defer func() {
		c.conn.Close()
		c.hub.unregister <- c
	}()
	for {
		//continiously read incoming websocket messages
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("read error: %v", err)
			break
		}

		if messageType == websocket.TextMessage {
			log.Printf("Recieved message: %s", string(payload))

		}

	}

}

func (c *Client) WritePump() {
	//clean up connection after loop break
	defer func() {
		c.conn.Close()
	}()

	for message := range c.send {
		//Write message to web socket
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			log.Printf("write error: %v", err)
			break
		}
	}

}

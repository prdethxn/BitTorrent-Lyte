package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Creates a running Hub and WebSocket test server.
func setupWebSocketTest(t *testing.T) (*Hub, *httptest.Server, string) {
	t.Helper()

	hub := NewHub()
	go hub.Run()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWs)

	server := httptest.NewServer(mux)

	t.Cleanup(server.Close)

	wsURL := "ws" + server.URL[len("http"):] + "/ws"

	return hub, server, wsURL
}

// Connects a WebSocket client to the test server.
func connectTestClient(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()

	dialer := websocket.Dialer{
		HandshakeTimeout: 3 * time.Second,
	}

	headers := http.Header{}
	headers.Set("Origin", "http://localhost:5173")

	conn, response, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if response != nil {
			t.Fatalf("WebSocket connection failed: %v (HTTP %d)", err, response.StatusCode)
		}
		t.Fatalf("WebSocket connection failed: %v", err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// Verifies that multiple clients can connect and receive broadcasts.
func TestBroadcastToMultipleClients(t *testing.T) {
	hub, _, wsURL := setupWebSocketTest(t)

	clientOne := connectTestClient(t, wsURL)
	clientTwo := connectTestClient(t, wsURL)

	event := Event{
		Type:    "download_started",
		Message: "Download started",
		Data: map[string]any{
			"piece_count": 10,
		},
	}

	hub.Broadcast(event)

	for i, conn := range []*websocket.Conn{clientOne, clientTwo} {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))

		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Client %d failed to receive broadcast: %v", i+1, err)
		}

		var received Event

		if err := json.Unmarshal(message, &received); err != nil {
			t.Fatalf("Client %d received invalid JSON: %v", i+1, err)
		}

		if received.Type != event.Type {
			t.Errorf("Client %d: expected event type %q, got %q",
				i+1, event.Type, received.Type)
		}

		if received.Message != event.Message {
			t.Errorf("Client %d: expected message %q, got %q",
				i+1, event.Message, received.Message)
		}
	}
}

// Verifies that disconnected clients can be unregistered safely.
func TestClientUnregistration(t *testing.T) {
	hub, _, _ := setupWebSocketTest(t)

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}

	// Register the client.
	hub.register <- client

	// Unregister the client.
	hub.unregister <- client

	// The Hub should close the client's outgoing channel.
	select {
	case _, open := <-client.send:
		if open {
			t.Fatal("Expected client send channel to be closed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Client send channel was not closed")
	}
}

// Verifies that the Hub safely handles repeated unregistration.
func TestRepeatedUnregistration(t *testing.T) {
	hub, _, _ := setupWebSocketTest(t)

	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}

	hub.register <- client
	hub.unregister <- client

	// A second unregistration must not close the channel twice.
	hub.unregister <- client

	select {
	case _, open := <-client.send:
		if open {
			t.Fatal("Expected client send channel to remain closed")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Expected client send channel to be closed")
	}
}

// Verifies that concurrent broadcasts are delivered without data races
// or lost messages.
func TestConcurrentBroadcasts(t *testing.T) {
	hub, _, wsURL := setupWebSocketTest(t)

	conn := connectTestClient(t, wsURL)

	const messageCount = 50

	var wg sync.WaitGroup

	wg.Add(messageCount)

	for i := 0; i < messageCount; i++ {
		go func(index int) {
			defer wg.Done()

			hub.Broadcast(Event{
				Type:    "piece_downloaded",
				Message: fmt.Sprintf("Piece %d downloaded", index),
			})
		}(i)
	}

	wg.Wait()

	// Read all messages and verify that none were lost.
	received := make(map[string]bool)

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	for i := 0; i < messageCount; i++ {
		_, message, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("Failed to receive message %d: %v", i+1, err)
		}

		var event Event

		if err := json.Unmarshal(message, &event); err != nil {
			t.Fatalf("Invalid JSON received: %v", err)
		}

		received[event.Message] = true
	}

	if len(received) != messageCount {
		t.Fatalf("Expected %d unique messages, received %d",
			messageCount, len(received))
	}
}

// Verifies that clients can disconnect without crashing the Hub.
func TestClientDisconnect(t *testing.T) {
	hub, _, wsURL := setupWebSocketTest(t)

	conn := connectTestClient(t, wsURL)

	// Disconnect the client.
	if err := conn.Close(); err != nil {
		t.Fatalf("Failed to close WebSocket connection: %v", err)
	}

	// Give the read pump time to detect disconnection and unregister.
	time.Sleep(50 * time.Millisecond)

	// Broadcasting after disconnection must remain safe.
	hub.Broadcast(Event{
		Type:    "client_status",
		Message: "Testing disconnected client handling",
	})
}

// Verifies that events containing invalid JSON data are rejected
// without crashing the broadcaster.
func TestInvalidEventData(t *testing.T) {
	hub, _, wsURL := setupWebSocketTest(t)

	conn := connectTestClient(t, wsURL)

	// Channels cannot be encoded as JSON.
	hub.Broadcast(Event{
		Type: "invalid_event",
		Data: make(chan int),
	})

	// A subsequent valid event must still be delivered.
	hub.Broadcast(Event{
		Type:    "valid_event",
		Message: "Broadcaster still operational",
	})

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	_, message, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to receive valid event: %v", err)
	}

	var received Event

	if err := json.Unmarshal(message, &received); err != nil {
		t.Fatalf("Failed to decode valid event: %v", err)
	}

	if received.Type != "valid_event" {
		t.Fatalf("Expected valid_event, got %q", received.Type)
	}
}

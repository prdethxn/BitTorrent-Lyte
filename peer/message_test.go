package peer

import (
	"encoding/binary"
	"net"
	"testing"
)

func createTestMessage(id byte, payload []byte) []byte {
	length := uint32(1 + len(payload))
	buffer := make([]byte, 4+length)

	binary.BigEndian.PutUint32(buffer[:4], length)
	buffer[4] = id
	copy(buffer[5:], payload)

	return buffer
}

func TestReadMessageSuccess(t *testing.T) {
	expectedID := byte(5)
	expectedPayload := []byte{0x01, 0x02, 0x03}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Write(createTestMessage(expectedID, expectedPayload))
	}()

	message, err := ReadMessage(server)
	if err != nil {
		t.Fatalf("ReadMessage() returned unexpected error: %v", err)
	}

	if message.Length != 4 {
		t.Errorf("expected length 4, got %d", message.Length)
	}

	if message.ID != expectedID {
		t.Errorf("expected ID %d, got %d", expectedID, message.ID)
	}

	if string(message.Payload) != string(expectedPayload) {
		t.Errorf("incorrect payload returned")
	}
}

func TestReadMessageKeepAlive(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Write([]byte{0x00, 0x00, 0x00, 0x00})
	}()

	message, err := ReadMessage(server)
	if err != nil {
		t.Fatalf("unexpected error reading keep-alive: %v", err)
	}

	if message.Length != 0 {
		t.Errorf("expected length 0, got %d", message.Length)
	}

	if message.ID != 0 || len(message.Payload) != 0 {
		t.Error("expected empty keep-alive message")
	}
}

func TestReadMessageFragmented(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestMessage(5, []byte{0x01, 0x02, 0x03})

	go func() {
		for i := 0; i < len(data); i += 2 {
			end := i + 2
			if end > len(data) {
				end = len(data)
			}

			if _, err := client.Write(data[i:end]); err != nil {
				return
			}
		}
	}()

	message, err := ReadMessage(server)
	if err != nil {
		t.Fatalf("failed to read fragmented message: %v", err)
	}

	if message.ID != 5 {
		t.Errorf("expected ID 5, got %d", message.ID)
	}

	if len(message.Payload) != 3 {
		t.Errorf("expected payload length 3, got %d", len(message.Payload))
	}
}

func TestReadMessageSequential(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	first := createTestMessage(5, []byte{0x01})
	second := createTestMessage(2, []byte{0xAA, 0xBB})

	go func() {
		client.Write(first)
		client.Write(second)
	}()

	firstMessage, err := ReadMessage(server)
	if err != nil {
		t.Fatalf("failed to read first message: %v", err)
	}

	secondMessage, err := ReadMessage(server)
	if err != nil {
		t.Fatalf("failed to read second message: %v", err)
	}

	if firstMessage.ID != 5 {
		t.Errorf("expected first message ID 5, got %d", firstMessage.ID)
	}

	if secondMessage.ID != 2 {
		t.Errorf("expected second message ID 2, got %d", secondMessage.ID)
	}

	if len(firstMessage.Payload) != 1 {
		t.Errorf("incorrect first message payload length")
	}

	if len(secondMessage.Payload) != 2 {
		t.Errorf("incorrect second message payload length")
	}
}

func TestReadMessageOversized(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	oversizedLength := make([]byte, 4)
	binary.BigEndian.PutUint32(oversizedLength, MaxMessageLength+1)

	go func() {
		client.Write(oversizedLength)
	}()

	_, err := ReadMessage(server)
	if err == nil {
		t.Fatal("expected error for oversized message")
	}
}

func TestReadMessageIncompleteLength(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	go func() {
		client.Write([]byte{0x00, 0x00})
		client.Close()
	}()

	_, err := ReadMessage(server)
	if err == nil {
		t.Fatal("expected error for incomplete length prefix")
	}
}

func TestReadMessageIncompletePayload(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	go func() {
		client.Write([]byte{0x00, 0x00, 0x00, 0x05, 0x01, 0x02})
		client.Close()
	}()

	_, err := ReadMessage(server)
	if err == nil {
		t.Fatal("expected error for incomplete message payload")
	}
}

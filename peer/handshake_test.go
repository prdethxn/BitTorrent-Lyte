package peer

import (
	"net"
	"testing"
)

func createTestHandshake(infoHash, peerID [20]byte) []byte {
	buffer := make([]byte, 68)

	buffer[0] = 19
	copy(buffer[1:20], "BitTorrent protocol")
	copy(buffer[28:48], infoHash[:])
	copy(buffer[48:68], peerID[:])

	return buffer
}

func TestReadHandshakeSuccess(t *testing.T) {
	var expectedHash [20]byte
	var expectedPeerID [20]byte

	copy(expectedHash[:], "12345678901234567890")
	copy(expectedPeerID[:], "ABCDEFGHIJKLMNOPQRST")

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Send a complete handshake through the connection.
	go func() {
		client.Write(createTestHandshake(expectedHash, expectedPeerID))
	}()

	handshake, err := readHandshake(server)
	if err != nil {
		t.Fatalf("readHandshake() returned unexpected error: %v", err)
	}

	if handshake.InfoHash != expectedHash {
		t.Errorf("incorrect info hash returned")
	}

	if handshake.PeerID != expectedPeerID {
		t.Errorf("incorrect peer ID returned")
	}
}

func TestReadHandshakeFragmented(t *testing.T) {
	var expectedHash [20]byte
	var expectedPeerID [20]byte

	copy(expectedHash[:], "12345678901234567890")
	copy(expectedPeerID[:], "ABCDEFGHIJKLMNOPQRST")

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestHandshake(expectedHash, expectedPeerID)

	// Send the handshake in smaller writes to simulate fragmentation.
	go func() {
		for i := 0; i < len(data); i += 5 {
			end := i + 5
			if end > len(data) {
				end = len(data)
			}

			if _, err := client.Write(data[i:end]); err != nil {
				return
			}
		}
	}()

	handshake, err := readHandshake(server)
	if err != nil {
		t.Fatalf("readHandshake() failed on fragmented input: %v", err)
	}

	if handshake.InfoHash != expectedHash {
		t.Error("incorrect info hash after fragmented read")
	}

	if handshake.PeerID != expectedPeerID {
		t.Error("incorrect peer ID after fragmented read")
	}
}

func TestReadHandshakeInvalidLength(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestHandshake([20]byte{}, [20]byte{})
	data[0] = 18

	go func() {
		client.Write(data)
	}()

	_, err := readHandshake(server)
	if err == nil {
		t.Fatal("expected error for invalid protocol length")
	}
}

func TestReadHandshakeInvalidProtocol(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestHandshake([20]byte{}, [20]byte{})
	copy(data[1:20], "Invalid protocol!!!")

	go func() {
		client.Write(data)
	}()

	_, err := readHandshake(server)
	if err == nil {
		t.Fatal("expected error for invalid protocol identifier")
	}
}

func TestReadHandshakeIncomplete(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	go func() {
		data := createTestHandshake([20]byte{}, [20]byte{})
		client.Write(data[:30])
		client.Close()
	}()

	_, err := readHandshake(server)
	if err == nil {
		t.Fatal("expected error for incomplete handshake")
	}
}

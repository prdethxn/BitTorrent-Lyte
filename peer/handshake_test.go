package peer

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// Creates a valid 80-byte custom handshake for testing.
func createTestHandshake(infoHash [32]byte, peerID [20]byte) []byte {
	buffer := make([]byte, 80)

	buffer[0] = 19
	copy(buffer[1:20], "BitTorrent protocol")
	copy(buffer[28:60], infoHash[:])
	copy(buffer[60:80], peerID[:])

	return buffer
}

// Tests that a complete handshake is correctly received and decoded.
func TestReadHandshakeSuccess(t *testing.T) {
	var expectedHash [32]byte
	var expectedPeerID [20]byte

	copy(expectedHash[:], "12345678901234567890123456789012")
	copy(expectedPeerID[:], "ABCDEFGHIJKLMNOPQRST")

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	// Send a complete handshake through the connection.
	go func() {
		client.Write(createTestHandshake(expectedHash, expectedPeerID))
	}()

	handshake, err := ReadHandshake(server)
	if err != nil {
		t.Fatalf("readHandshake() returned unexpected error: %v", err)
	}

	if handshake.InfoHash != expectedHash {
		t.Error("incorrect info hash returned")
	}

	if handshake.PeerID != expectedPeerID {
		t.Error("incorrect peer ID returned")
	}
}

// Tests that readHandshake correctly handles fragmented TCP data.
func TestReadHandshakeFragmented(t *testing.T) {
	var expectedHash [32]byte
	var expectedPeerID [20]byte

	copy(expectedHash[:], "12345678901234567890123456789012")
	copy(expectedPeerID[:], "ABCDEFGHIJKLMNOPQRST")

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestHandshake(expectedHash, expectedPeerID)

	// Send the handshake in smaller writes to simulate TCP fragmentation.
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

	handshake, err := ReadHandshake(server)
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

// Tests that an invalid protocol length is rejected.
func TestReadHandshakeInvalidLength(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestHandshake([32]byte{}, [20]byte{})
	data[0] = 18

	go func() {
		client.Write(data)
	}()

	_, err := ReadHandshake(server)
	if err == nil {
		t.Fatal("expected error for invalid protocol length")
	}
}

// Tests that an invalid protocol identifier is rejected.
func TestReadHandshakeInvalidProtocol(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	data := createTestHandshake([32]byte{}, [20]byte{})
	copy(data[1:20], "Invalid protocol!!!")

	go func() {
		client.Write(data)
	}()

	_, err := ReadHandshake(server)
	if err == nil {
		t.Fatal("expected error for invalid protocol identifier")
	}
}

// Tests that an incomplete handshake returns an error.
func TestReadHandshakeIncomplete(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()

	go func() {
		data := createTestHandshake([32]byte{}, [20]byte{})
		client.Write(data[:30])
		client.Close()
	}()

	_, err := ReadHandshake(server)
	if err == nil {
		t.Fatal("expected error for incomplete handshake")
	}
}

type mockWriteConn struct {
	writeFunc func([]byte) (int, error)
}

func (m *mockWriteConn) Write(p []byte) (int, error) {
	return m.writeFunc(p)
}

func (m *mockWriteConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (m *mockWriteConn) Close() error                     { return nil }
func (m *mockWriteConn) LocalAddr() net.Addr              { return nil }
func (m *mockWriteConn) RemoteAddr() net.Addr             { return nil }
func (m *mockWriteConn) SetDeadline(time.Time) error      { return nil }
func (m *mockWriteConn) SetReadDeadline(time.Time) error  { return nil }
func (m *mockWriteConn) SetWriteDeadline(time.Time) error { return nil }

// Tests that writeFull successfully writes all bytes.
func TestWriteFullSuccess(t *testing.T) {
	data := []byte("BitTorrent")

	connection := &mockWriteConn{
		writeFunc: func(p []byte) (int, error) {
			return len(p), nil
		},
	}
	if err := WriteFull(connection, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Tests that writeFull handles partial writes.
func TestWriteFullPartialWrites(t *testing.T) {
	data := []byte("BitTorrent")
	totalWritten := 0

	connection := &mockWriteConn{
		writeFunc: func(p []byte) (int, error) {
			n := min(3, len(p))
			totalWritten += n
			return n, nil
		},
	}

	if err := WriteFull(connection, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if totalWritten != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), totalWritten)
	}
}

// Tests that writeFull rejects zero-byte writes.
func TestWriteFullZeroBytes(t *testing.T) {
	connection := &mockWriteConn{
		writeFunc: func(p []byte) (int, error) {
			return 0, nil
		},
	}

	if err := WriteFull(connection, []byte("test")); err == nil {
		t.Fatal("expected error for zero-byte write")
	}
}

// Tests that writeFull returns connection errors.
func TestWriteFullConnectionError(t *testing.T) {
	expectedErr := errors.New("connection failed")

	connection := &mockWriteConn{
		writeFunc: func(p []byte) (int, error) {
			return 0, expectedErr
		},
	}

	err := WriteFull(connection, []byte("test"))

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected connection error, got: %v", err)
	}
}

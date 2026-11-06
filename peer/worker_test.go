package peer

import (
	"io"
	"net"
	"testing"
	"time"
)

// Creates a valid 80-byte handshake for testing.
func makeTestHandshake(infoHash [32]byte, peerID [20]byte) []byte {
	buffer := make([]byte, 80)

	buffer[0] = 19
	copy(buffer[1:20], "BitTorrent protocol")
	copy(buffer[28:60], infoHash[:])
	copy(buffer[60:80], peerID[:])

	return buffer
}

// Starts a local peer that accepts a connection, reads the client's
// handshake, and responds with the supplied handshake data.
func startTestPeer(t *testing.T, response []byte) (string, <-chan error) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test peer: %v", err)
	}

	t.Cleanup(func() {
		listener.Close()
	})

	serverErr := make(chan error, 1)

	go func() {
		connection, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer connection.Close()

		// Prevent the test server from hanging indefinitely.
		connection.SetDeadline(time.Now().Add(3 * time.Second))

		// Read the client's complete handshake.
		clientHandshake := make([]byte, 80)
		if _, err := io.ReadFull(connection, clientHandshake); err != nil {
			serverErr <- err
			return
		}

		// Send the remote peer's handshake.
		if response != nil {
			if _, err := connection.Write(response); err != nil {
				serverErr <- err
				return
			}
		}

		serverErr <- nil
	}()

	return listener.Addr().String(), serverErr
}

// Test that a worker successfully connects and completes a valid handshake.
func TestProcessJobSuccessfulHandshake(t *testing.T) {
	var infoHash [32]byte
	var peerID [20]byte

	infoHash[0] = 10
	peerID[0] = 20

	expectedHandshake := makeTestHandshake(infoHash, peerID)

	address, serverErr := startTestPeer(t, expectedHandshake)

	job := PieceJob{
		PeerAddress: address,
		Infohash:    infoHash,
		LocalPeerID: peerID,
	}

	err := processJob(job)

	if err != nil {
		t.Fatalf("expected successful handshake, got error: %v", err)
	}

	// Ensure the test peer successfully received the handshake.
	if err := <-serverErr; err != nil {
		t.Fatalf("test peer failed: %v", err)
	}
}

// Test that connecting to an unavailable peer returns an error.
func TestProcessJobConnectionFailure(t *testing.T) {
	// Reserve an available local TCP port, then release it.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve test port: %v", err)
	}

	address := listener.Addr().String()
	listener.Close()

	job := PieceJob{
		PeerAddress: address,
	}

	err = processJob(job)

	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
}

// Test that closing the connection before receiving a handshake
// causes the job to fail.
func TestProcessJobHandshakeFailure(t *testing.T) {
	address, serverErr := startTestPeer(t, nil)

	job := PieceJob{
		PeerAddress: address,
	}

	err := processJob(job)

	if err == nil {
		t.Fatal("expected handshake read error, got nil")
	}

	if serverErr := <-serverErr; serverErr != nil {
		t.Fatalf("test peer failed to receive client handshake: %v", serverErr)
	}
}

// Test that a handshake with an invalid protocol header is rejected.
func TestProcessJobInvalidProtocol(t *testing.T) {
	var infoHash [32]byte
	var peerID [20]byte

	response := makeTestHandshake(infoHash, peerID)

	// Corrupt the protocol identifier.
	response[1] = 'X'

	address, serverErr := startTestPeer(t, response)

	job := PieceJob{
		PeerAddress: address,
		Infohash:    infoHash,
		LocalPeerID: peerID,
	}

	err := processJob(job)

	if err == nil {
		t.Fatal("expected invalid protocol error, got nil")
	}

	if serverErr := <-serverErr; serverErr != nil {
		t.Fatalf("test peer failed: %v", serverErr)
	}
}

// Test that a peer advertising a different InfoHash is rejected.
func TestProcessJobInfoHashMismatch(t *testing.T) {
	var localHash [32]byte
	var remoteHash [32]byte
	var peerID [20]byte

	localHash[0] = 10
	remoteHash[0] = 99

	response := makeTestHandshake(remoteHash, peerID)

	address, serverErr := startTestPeer(t, response)

	job := PieceJob{
		PeerAddress: address,
		Infohash:    localHash,
		LocalPeerID: peerID,
	}

	err := processJob(job)

	if err == nil {
		t.Fatal("expected InfoHash mismatch error, got nil")
	}

	if err.Error() != "infohash mismatch" {
		t.Fatalf("expected InfoHash mismatch, got: %v", err)
	}

	if serverErr := <-serverErr; serverErr != nil {
		t.Fatalf("test peer failed: %v", serverErr)
	}
}

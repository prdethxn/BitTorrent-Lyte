package peer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// Creates a valid 80-byte BitTorrent handshake for testing.
func makeTestHandshake(infoHash [32]byte, peerID [20]byte) []byte {
	buffer := make([]byte, 80)

	buffer[0] = 19
	copy(buffer[1:20], "BitTorrent protocol")
	copy(buffer[28:60], infoHash[:])
	copy(buffer[60:80], peerID[:])
	return buffer
}

// Starts a mock peer that accepts a connection, reads the client’s
// handshake and responds with the supplied handshake.
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
		connection.SetDeadline(time.Now().Add(5 * time.Second))
		clientHandshake := make([]byte, 80)
		if _, err := io.ReadFull(connection, clientHandshake); err != nil {
			serverErr <- err
			return
		}
		if response != nil {
			if err := WriteFull(connection, response); err != nil {
				serverErr <- err
				return
			}
		}
		serverErr <- nil
	}()
	return listener.Addr().String(), serverErr
}

// Handles one mock peer connection, including handshake and piece requests.
// The mock peer returns the requested block from its supplied piece data.
func handlePieceConnection(
	connection net.Conn,
	infoHash [32]byte,
	pieceIndex uint32,
	pieceData []byte,
) error {
	defer connection.Close()

	if err := connection.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	clientHandshake := make([]byte, 80)
	if _, err := io.ReadFull(connection, clientHandshake); err != nil {
		return err
	}
	if err := WriteFull(connection, makeTestHandshake(infoHash, [20]byte{1})); err != nil {
		return err
	}
	// Read the request message and extract its piece coordinates.
	request := make([]byte, 17)
	if _, err := io.ReadFull(connection, request); err != nil {
		return err
	}
	if binary.BigEndian.Uint32(request[:4]) != 13 || request[4] != MsgRequest {
		return fmt.Errorf("invalid piece request")
	}
	requestedIndex := binary.BigEndian.Uint32(request[5:9])
	begin := binary.BigEndian.Uint32(request[9:13])
	length := binary.BigEndian.Uint32(request[13:17])
	if requestedIndex != pieceIndex {
		return fmt.Errorf("unexpected piece index")
	}
	if length == 0 || uint64(begin)+uint64(length) > uint64(len(pieceData)) {
		return fmt.Errorf("invalid requested block coordinates")
	}
	block := pieceData[begin : begin+length]
	// Construct a BitTorrent piece response (message ID 7).
	response := make([]byte, 13+len(block))
	binary.BigEndian.PutUint32(response[:4], uint32(9+len(block)))
	response[4] = 7
	binary.BigEndian.PutUint32(response[5:9], pieceIndex)
	binary.BigEndian.PutUint32(response[9:13], begin)
	copy(response[13:], block)
	return WriteFull(connection, response)
}

// Starts a mock peer that can handle multiple independent connections.
// Each connection is handled concurrently to simulate a real peer.
func startPieceTestPeer(
	t *testing.T,
	connectionCount int,
	infoHash [32]byte,
	pieceIndex uint32,
	pieceData []byte,
) (string, <-chan error) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start piece peer: %v", err)
	}
	t.Cleanup(func() {
		listener.Close()
	})
	serverErrors := make(chan error, connectionCount)
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < connectionCount; i++ {
			connection, err := listener.Accept()
			if err != nil {
				serverErrors <- err
				return
			}
			wg.Add(1)
			go func(connection net.Conn) {
				defer wg.Done()
				serverErrors <- handlePieceConnection(
					connection,
					infoHash,
					pieceIndex,
					pieceData,
				)
			}(connection)
		}
		wg.Wait()
	}()
	return listener.Addr().String(), serverErrors
}

// Test that a complete job successfully connects, performs a handshake,
// downloads its assigned piece and verifies its SHA-256 hash.
func TestProcessJobSuccessfulDownload(t *testing.T) {
	var infoHash [32]byte
	infoHash[0] = 10

	pieceData := []byte("Hello BitTorrent P2P!")
	hash := sha256.Sum256(pieceData)
	address, serverErr := startPieceTestPeer(
		t, 1, infoHash, 0, pieceData,
	)
	job := PieceJob{
		PieceIndex:   0,
		Infohash:     infoHash,
		PeerAddress:  address,
		PieceLength:  uint32(len(pieceData)),
		ExpectedHash: hex.EncodeToString(hash[:]),
	}
	result, err := processJob(job)
	if err != nil || result.Err != nil {
		t.Fatalf("expected successful job, got: %v", err)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("mock peer failed: %v", err)
	}
}

// Test that a downloaded piece with an incorrect SHA-256 hash is rejected.
func TestProcessJobIntegrityFailure(t *testing.T) {
	var infoHash [32]byte

	pieceData := []byte("Downloaded data")
	address, serverErr := startPieceTestPeer(
		t, 1, infoHash, 0, pieceData,
	)
	job := PieceJob{
		PieceIndex:   0,
		Infohash:     infoHash,
		PeerAddress:  address,
		PieceLength:  uint32(len(pieceData)),
		ExpectedHash: "incorrect_hash",
	}
	if _, err := processJob(job); err == nil {
		t.Fatal("expected integrity failure, got nil")
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("mock peer failed: %v", err)
	}
}

// Test that a worker retrieves and processes multiple jobs from its queue.
func TestPieceWorkerProcessesMultipleJobs(t *testing.T) {
	var infoHash [32]byte

	pieceData := []byte("Worker pool test")
	hash := sha256.Sum256(pieceData)
	const jobCount = 3
	address, serverErr := startPieceTestPeer(
		t, jobCount, infoHash, 0, pieceData,
	)
	jobQueue := makeJobQueue()
	resultsChannel := make(chan PieceResult, jobCount)
	done := make(chan struct{})
	go func() {
		pieceWorker(jobQueue, resultsChannel)
		close(done)
	}()
	for i := 0; i < jobCount; i++ {
		addJob(jobQueue, PieceJob{
			PieceIndex:   0,
			Infohash:     infoHash,
			PeerAddress:  address,
			PieceLength:  uint32(len(pieceData)),
			ExpectedHash: hex.EncodeToString(hash[:]),
		})
	}
	close(jobQueue)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("worker failed to finish processing queued jobs")
	}
	for i := 0; i < jobCount; i++ {
		if err := <-serverErr; err != nil {
			t.Fatalf("job %d failed: %v", i, err)
		}
	}
}

// Test that multiple goroutine workers independently process jobs
// concurrently using separate TCP connections.
func TestMultipleWorkersProcessJobsConcurrently(t *testing.T) {
	const workerCount = 3

	var infoHash [32]byte
	pieceData := []byte("Concurrent download test")
	hash := sha256.Sum256(pieceData)
	address, serverErr := startPieceTestPeer(
		t, workerCount, infoHash, 0, pieceData,
	)
	jobQueue := make(chan PieceJob, workerCount)
	resultsChannel := make(chan PieceResult, workerCount)
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pieceWorker(jobQueue, resultsChannel)
		}()
	}
	for i := 0; i < workerCount; i++ {
		addJob(jobQueue, PieceJob{
			PieceIndex:   0,
			Infohash:     infoHash,
			PeerAddress:  address,
			PieceLength:  uint32(len(pieceData)),
			ExpectedHash: hex.EncodeToString(hash[:]),
		})
	}
	close(jobQueue)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("workers failed to finish processing jobs")
	}
	for i := 0; i < workerCount; i++ {
		if err := <-serverErr; err != nil {
			t.Fatalf("concurrent job %d failed: %v", i, err)
		}
	}
}

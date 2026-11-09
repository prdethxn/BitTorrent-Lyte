package client

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/prdethxn/BitTorrent-Lyte/bitfield"
	"github.com/prdethxn/BitTorrent-Lyte/chunker"
	"github.com/prdethxn/BitTorrent-Lyte/peer"
)

func TestAnnounceSendsParametersAndDecodesResponse(t *testing.T) {
	want := TrackerResponse{
		TransactionId: 42,
		Peers: []TrackerPeer{
			{PeerID: "remote-peer", IPAddress: "127.0.0.1", Port: 6881},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		query := r.URL.Query()
		checks := map[string]string{
			"peer_id":        "local-peer",
			"port":           "6882",
			"info_hash":      "0123456789abcdef",
			"transaction_id": "42",
			"event":           "started",
		}
		for key, expected := range checks {
			if got := query.Get(key); got != expected {
				t.Errorf("query %q = %q, want %q", key, got, expected)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	got, err := Announce(server.URL+"/announce", "local-peer", 6882,
		"0123456789abcdef", 42, "started")
	if err != nil {
		t.Fatalf("Announce returned error: %v", err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Fatalf("Announce response = %#v, want %#v", got, &want)
	}
}

func TestAnnounceRejectsBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "tracker unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := Announce(server.URL, "peer", 6881, "hash", 1, "started")
	if err == nil {
		t.Fatal("Announce succeeded for a non-200 tracker response")
	}
}

func TestAnnounceRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "{not-json")
	}))
	defer server.Close()

	_, err := Announce(server.URL, "peer", 6881, "hash", 1, "started")
	if err == nil {
		t.Fatal("Announce succeeded for an invalid JSON response")
	}
}

func TestAnnounceRejectsInvalidURL(t *testing.T) {
	_, err := Announce("http://[::1", "peer", 6881, "hash", 1, "started")
	if err == nil {
		t.Fatal("Announce succeeded for an invalid URL")
	}
}

func TestCreatePieceJobsDistributesPiecesAndCalculatesFinalLength(t *testing.T) {
	var infoHash [32]byte
	infoHash[0] = 0xab
	var localPeerID [20]byte
	localPeerID[0] = 0xcd

	response := &TrackerResponse{
		Peers: []TrackerPeer{
			{PeerID: "peer-one", IPAddress: "127.0.0.1", Port: 6881},
			{PeerID: "peer-two", IPAddress: "::1", Port: 6882},
		},
	}
	metadata := chunker.TorrentMetaData{
		Filename:    "sample.bin",
		TotalLength: 10,
		PieceLength: 4,
		PieceHashes: []string{"hash-0", "hash-1", "hash-2"},
	}

	jobs, err := CreatePieceJobs(response, metadata, infoHash, localPeerID)
	if err != nil {
		t.Fatalf("CreatePieceJobs returned error: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("got %d jobs, want 3", len(jobs))
	}

	wantAddresses := []string{
		net.JoinHostPort("127.0.0.1", strconv.Itoa(6881)),
		net.JoinHostPort("::1", strconv.Itoa(6882)),
		net.JoinHostPort("127.0.0.1", strconv.Itoa(6881)),
	}
	wantLengths := []uint32{4, 4, 2}

	for i, job := range jobs {
		if job.PieceIndex != uint32(i) {
			t.Errorf("job %d has piece index %d", i, job.PieceIndex)
		}
		if job.PeerAddress != wantAddresses[i] {
			t.Errorf("job %d address = %q, want %q", i, job.PeerAddress, wantAddresses[i])
		}
		if job.PieceLength != wantLengths[i] {
			t.Errorf("job %d length = %d, want %d", i, job.PieceLength, wantLengths[i])
		}
		if job.Infohash != infoHash {
			t.Errorf("job %d has incorrect infohash", i)
		}
		if job.LocalPeerID != localPeerID {
			t.Errorf("job %d has incorrect local peer ID", i)
		}
		if job.ExpectedHash != metadata.PieceHashes[i] {
			t.Errorf("job %d expected hash = %q, want %q", i, job.ExpectedHash, metadata.PieceHashes[i])
		}
	}
}

func TestCreatePieceJobsRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata chunker.TorrentMetaData
	}{
		{
			name: "zero piece length",
			metadata: chunker.TorrentMetaData{
				TotalLength: 1, PieceLength: 0, PieceHashes: []string{"hash"},
			},
		},
		{
			name: "negative total length",
			metadata: chunker.TorrentMetaData{
				TotalLength: -1, PieceLength: 4,
			},
		},
		{
			name: "incorrect hash count",
			metadata: chunker.TorrentMetaData{
				TotalLength: 5, PieceLength: 4, PieceHashes: []string{"only-one"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CreatePieceJobs(&TrackerResponse{
				Peers: []TrackerPeer{{IPAddress: "127.0.0.1", Port: 6881}},
			}, test.metadata, [32]byte{}, [20]byte{})
			if err == nil {
				t.Fatal("CreatePieceJobs succeeded with invalid metadata")
			}
		})
	}
}

func TestCreatePieceJobsRejectsMissingPeersForNonEmptyFile(t *testing.T) {
	metadata := chunker.TorrentMetaData{
		TotalLength: 1, PieceLength: 4, PieceHashes: []string{"hash"},
	}
	_, err := CreatePieceJobs(&TrackerResponse{}, metadata, [32]byte{}, [20]byte{})
	if err == nil {
		t.Fatal("CreatePieceJobs succeeded without available peers")
	}
}

func TestCreatePieceJobsAllowsEmptyFileWithoutPeers(t *testing.T) {
	metadata := chunker.TorrentMetaData{
		TotalLength: 0, PieceLength: 4, PieceHashes: []string{},
	}

	jobs, err := CreatePieceJobs(nil, metadata, [32]byte{}, [20]byte{})
	if err != nil {
		t.Fatalf("CreatePieceJobs rejected empty file: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("got %d jobs for empty file, want 0", len(jobs))
	}
}

func TestHandleUploadSendsRequestedBlock(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "seed.bin")
	fileData := []byte("abcdefgh")
	if err := os.WriteFile(filePath, fileData, 0600); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	available := bitfield.NewBitfield(2)
	if err := bitfield.MarkAsDownloaded(available, 0); err != nil {
		t.Fatalf("mark piece available: %v", err)
	}

	serverConn, clientConn := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- handleUpload(serverConn, filePath, 4, available)
	}()

	// Request bytes 1-2 from piece zero.
	request := make([]byte, 17)
	binary.BigEndian.PutUint32(request[0:4], 13)
	request[4] = peer.MsgRequest
	binary.BigEndian.PutUint32(request[5:9], 0) // piece index
	binary.BigEndian.PutUint32(request[9:13], 1) // offset
	binary.BigEndian.PutUint32(request[13:17], 2) // block length

	if err := peer.WriteFull(clientConn, request); err != nil {
		clientConn.Close()
		t.Fatalf("write piece request: %v", err)
	}

	message, err := peer.ReadMessage(clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("read piece response: %v", err)
	}
	if message.ID != peer.MsgPiece {
		t.Fatalf("response message ID = %d, want %d", message.ID, peer.MsgPiece)
	}
	if len(message.Payload) != 10 {
		t.Fatalf("response payload length = %d, want 10", len(message.Payload))
	}
	if got := binary.BigEndian.Uint32(message.Payload[0:4]); got != 0 {
		t.Errorf("response piece index = %d, want 0", got)
	}
	if got := binary.BigEndian.Uint32(message.Payload[4:8]); got != 1 {
		t.Errorf("response offset = %d, want 1", got)
	}
	if got, want := string(message.Payload[8:]), "bc"; got != want {
		t.Errorf("response data = %q, want %q", got, want)
	}

	// Closing the client side ends the server's request-reading loop.
	_ = clientConn.Close()
	if err := <-done; err != nil {
		t.Fatalf("upload handler returned error after clean close: %v", err)
	}
}

func TestHandleUploadRejectsUnavailablePiece(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "seed.bin")
	if err := os.WriteFile(filePath, []byte("abcdefgh"), 0600); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	available := bitfield.NewBitfield(2) // Neither piece has been marked available.
	serverConn, clientConn := net.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- handleUpload(serverConn, filePath, 4, available)
	}()

	request := make([]byte, 17)
	binary.BigEndian.PutUint32(request[0:4], 13)
	request[4] = peer.MsgRequest
	binary.BigEndian.PutUint32(request[5:9], 0)
	binary.BigEndian.PutUint32(request[9:13], 0)
	binary.BigEndian.PutUint32(request[13:17], 1)

	if err := peer.WriteFull(clientConn, request); err != nil {
		clientConn.Close()
		t.Fatalf("write piece request: %v", err)
	}
	_ = clientConn.Close()

	if err := <-done; err == nil {
		t.Fatal("upload handler accepted a request for an unavailable piece")
	}
}

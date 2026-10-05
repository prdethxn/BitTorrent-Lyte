package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestAnnounceValidRequest(t *testing.T) {
	store := makeSwarmStore()
	handler := makeHandler(store)

	req := httptest.NewRequest(
		http.MethodGet,
		"/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=42&event=started",
		nil,
	)

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	if rec.Body.String() != "42" {
		t.Errorf("expected transaction ID 42, got %q", rec.Body.String())
	}
}

func TestAnnounceValidEvents(t *testing.T) {
	events := []string{"started", "completed", "stopped"}

	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			store := makeSwarmStore()
			handler := makeHandler(store)

			req := httptest.NewRequest(
				http.MethodGet,
				"/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=42&event="+event,
				nil,
			)

			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
			}
		})
	}
}

func TestAnnounceMissingParameter(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "missing port",
			url:  "/announce?info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=42&event=started",
		},
		{
			name: "missing info_hash",
			url:  "/announce?port=6881&peer_id=peer123&transaction_id=42&event=started",
		},
		{
			name: "missing peer_id",
			url:  "/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&transaction_id=42&event=started",
		},
		{
			name: "missing transaction_id",
			url:  "/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&event=started",
		},
		{
			name: "missing event",
			url:  "/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=42",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := makeSwarmStore()
			handler := makeHandler(store)

			req := httptest.NewRequest(http.MethodGet, test.url, nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestAnnounceInvalidPort(t *testing.T) {
	tests := []string{
		"abc",
		"0",
		"-1",
		"65536",
	}

	for _, port := range tests {
		t.Run(port, func(t *testing.T) {
			store := makeSwarmStore()
			handler := makeHandler(store)

			req := httptest.NewRequest(
				http.MethodGet,
				"/announce?port="+port+"&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=42&event=started",
				nil,
			)

			rec := httptest.NewRecorder()

			handler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}
		})
	}
}

func TestAnnounceInvalidInfoHash(t *testing.T) {
	store := makeSwarmStore()
	handler := makeHandler(store)

	req := httptest.NewRequest(
		http.MethodGet,
		"/announce?port=6881&info_hash=not-a-valid-hash&peer_id=peer123&transaction_id=42&event=started",
		nil,
	)

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAnnounceEmptyPeerID(t *testing.T) {
	store := makeSwarmStore()
	handler := makeHandler(store)

	req := httptest.NewRequest(
		http.MethodGet,
		"/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=&transaction_id=42&event=started",
		nil,
	)

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAnnounceInvalidTransactionID(t *testing.T) {
	store := makeSwarmStore()
	handler := makeHandler(store)

	req := httptest.NewRequest(
		http.MethodGet,
		"/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=abc&event=started",
		nil,
	)

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestAnnounceInvalidEvent(t *testing.T) {
	store := makeSwarmStore()
	handler := makeHandler(store)

	req := httptest.NewRequest(
		http.MethodGet,
		"/announce?port=6881&info_hash=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&peer_id=peer123&transaction_id=42&event=invalid",
		nil,
	)

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestGetOrCreateSwarm(t *testing.T) {
	store := makeSwarmStore()

	firstSwarm := getOrCreateSwarm(store, "hash123")
	secondSwarm := getOrCreateSwarm(store, "hash123")

	if firstSwarm != secondSwarm {
		t.Error("expected the same swarm to be returned for the same info_hash")
	}

	if firstSwarm.info_hash != "hash123" {
		t.Errorf("expected info_hash hash123, got %s", firstSwarm.info_hash)
	}
}

func TestAddMultiplePeersToSwarm(t *testing.T) {
	store := makeSwarmStore()
	mySwarm := getOrCreateSwarm(store, "hash123")

	peerOne := makePeer("peer1", 6881)
	peerTwo := makePeer("peer2", 6882)

	addPeerToSwarm(mySwarm, peerOne)
	addPeerToSwarm(mySwarm, peerTwo)

	if len(mySwarm.peerCollection) != 2 {
		t.Errorf("expected 2 peers, got %d", len(mySwarm.peerCollection))
	}
}

func TestAddPeerUpdatesExistingPeer(t *testing.T) {
	store := makeSwarmStore()
	mySwarm := getOrCreateSwarm(store, "hash123")

	firstPeer := makePeer("peer1", 6881)
	secondPeer := makePeer("peer1", 6882)

	addPeerToSwarm(mySwarm, firstPeer)
	addPeerToSwarm(mySwarm, secondPeer)

	if len(mySwarm.peerCollection) != 1 {
		t.Errorf("expected 1 peer, got %d", len(mySwarm.peerCollection))
	}

	currentPeer := getPeer(mySwarm, "peer1")

	if currentPeer.port != 6882 {
		t.Errorf("expected updated port 6882, got %d", currentPeer.port)
	}
}

func TestConcurrentSwarmAndPeerAccess(t *testing.T) {
	store := makeSwarmStore()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			mySwarm := getOrCreateSwarm(store, "hash123")
			myPeer := makePeer(fmt.Sprintf("peer%d", i), 6881+i)

			addPeerToSwarm(mySwarm, myPeer)
			getPeer(mySwarm, myPeer.peerId)
		}(i)
	}

	wg.Wait()

	mySwarm := getSwarm(store, "hash123")

	if mySwarm == nil {
		t.Fatal("expected swarm to exist")
	}

	if len(mySwarm.peerCollection) != 100 {
		t.Errorf("expected 100 peers, got %d", len(mySwarm.peerCollection))
	}
}

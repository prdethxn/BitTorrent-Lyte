package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnnounceValidRequest(t *testing.T) {
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

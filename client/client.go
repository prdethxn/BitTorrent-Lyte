package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type TrackerResponse struct {
	TransactionId int           `json:"transaction_id"`
	Peers         []TrackerPeer `json:"peers"`
}

type TrackerPeer struct {
	PeerID    string `json:"peer_id"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port"`
}

// Make GET request to tracker service
func Announce(trackerURL string, peerID string, port int, infoHash string, transactionID int, event string) (*TrackerResponse, error) {
	//Parse tracker URL
	baseURL, err := url.Parse(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid tracker URL: %w", err)
	}

	//Get geury paramter map and set params
	params := baseURL.Query()
	params.Set("peer_id", peerID)
	params.Set("port", strconv.Itoa(port))
	params.Set("info_hash", infoHash)
	params.Set("transaction_id", strconv.Itoa(transactionID))
	params.Set("event", event)

	//Bind the parameters to the URL object
	baseURL.RawQuery = params.Encode()
	client := &http.Client{Timeout: 5 * time.Second}
	//Send GET request
	request, err := http.NewRequest("GET", baseURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("tracker request failed: %w", err)
	}

	response, err := client.Do(request)

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned bad status: %s", response.Status)
	}

	if err != nil {
		return nil, fmt.Errorf("tracker request failed: %w", err)
	}
	defer response.Body.Close() //safley close open connection stream

	//Decode JSON response into the struct
	var trackerResponse TrackerResponse
	if err := json.NewDecoder(response.Body).Decode(&trackerResponse); err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %w", err)
	}

	return &trackerResponse, nil

}

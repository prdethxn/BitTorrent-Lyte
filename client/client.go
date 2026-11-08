package client

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/prdethxn/BitTorrent-Lyte/peer"
	"github.com/prdethxn/BitTorrent-Lyte/storage"
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

func DownloadFile(jobs []peer.PieceJob, workerCount int, outputPath string, pieceLength int, totalLength int64) error {
	pieces, failedPieces, err := peer.DownloadPieces(jobs, workerCount)

	if err != nil {
		return err
	}

	if len(failedPieces) > 0 {
		return fmt.Errorf("%d pieces failed to download", len(failedPieces))
	}

	err = storage.AssembleFile(outputPath, pieces, pieceLength, totalLength)
	if err != nil {
		return fmt.Errorf("failed to assemble file: %w", err)
	}
	return nil
}

func handleUpload(connection net.Conn, filePath string, pieceLegnth uint32) error {
	//read message and identify piece request
	message, err := peer.ReadMessage(connection)
	if err != nil {
		return err
	}

	if len(message.Payload) != 12 {
		return fmt.Errorf("message is missing requested fields")
	}

	if message.ID != peer.MsgRequest {
		return fmt.Errorf("message id and msg request mismatch")
	}

	index := binary.BigEndian.Uint32(message.Payload[0:4])
	offset := binary.BigEndian.Uint32(message.Payload[4:8])
	length := binary.BigEndian.Uint32(message.Payload[8:12])

	//extract request piece info and retrieve desires bytes from local storage
	fileOffset := uint64(index)*uint64(pieceLegnth) + uint64(offset)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("Failed to open requested file")
	}
	defer file.Close()

	buffer := make([]byte, length)
	numBytesRead, err := file.ReadAt(buffer, int64(fileOffset))

	if numBytesRead != int(length) {
		return fmt.Errorf("incomplete block read")
	}

	if err != nil {
		return err
	}

	payload := make([]byte, 8+length)
	binary.BigEndian.PutUint32(payload[0:4], index)
	binary.BigEndian.PutUint32(payload[4:8], offset)
	copy(payload[8:], buffer)
	//construct and send msgPiece response
	msgPiece := peer.Message{
		Length:  uint32(1 + len(payload)),
		ID:      peer.MsgPiece,
		Payload: payload,
	}

	response := make([]byte, 4+msgPiece.Length)
	binary.BigEndian.PutUint32(response[0:4], msgPiece.Length)
	response[4] = msgPiece.ID
	copy(response[5:], msgPiece.Payload)

	return peer.WriteFull(connection, response)

}

package client

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/prdethxn/BitTorrent-Lyte/chunker"
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

func handleUpload(connection net.Conn, filePath string, pieceLength uint32) error {
	defer connection.Close()

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open requested file: %w", err)
	}
	defer file.Close()

	for {
		// Read the next incoming piece request.
		message, err := peer.ReadMessage(connection)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("failed to read piece request: %w", err)
		}

		// Ignore keep-alive messages.
		if message.Length == 0 {
			continue
		}

		// Ensure this is a valid piece request.
		if message.ID != peer.MsgRequest {
			return fmt.Errorf("unexpected message ID: %d", message.ID)
		}

		if len(message.Payload) != 12 {
			return fmt.Errorf("invalid piece request payload length")
		}

		index := binary.BigEndian.Uint32(message.Payload[0:4])
		offset := binary.BigEndian.Uint32(message.Payload[4:8])
		length := binary.BigEndian.Uint32(message.Payload[8:12])

		// Validate requested block length.
		if length == 0 || length > peer.BlockSize {
			return fmt.Errorf("invalid requested block length")
		}

		// Calculate absolute file offset.
		fileOffset := uint64(index)*uint64(pieceLength) + uint64(offset)

		// Validate requested block boundaries.
		if offset >= pieceLength || uint64(offset)+uint64(length) > uint64(pieceLength) {
			return fmt.Errorf("requested block exceeds piece boundaries")
		}

		// Retrieve requested bytes.
		buffer := make([]byte, length)
		numBytesRead, err := file.ReadAt(buffer, int64(fileOffset))
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read requested block: %w", err)
		}

		if numBytesRead != int(length) {
			return fmt.Errorf("incomplete block read")
		}

		// Construct piece response.
		payload := make([]byte, 8+length)
		binary.BigEndian.PutUint32(payload[0:4], index)
		binary.BigEndian.PutUint32(payload[4:8], offset)
		copy(payload[8:], buffer)

		response := make([]byte, 4+1+len(payload))
		binary.BigEndian.PutUint32(response[0:4], uint32(1+len(payload)))
		response[4] = peer.MsgPiece
		copy(response[5:], payload)

		// Send response before processing the next request.
		if err := peer.WriteFull(connection, response); err != nil {
			return fmt.Errorf("failed to send piece response: %w", err)
		}
	}
}

// Create download jobs by distributing pieces across available peers.
func CreatePieceJobs(trackerResponse *TrackerResponse, metadata chunker.TorrentMetaData, infoHash [32]byte, localPeerID [20]byte) ([]peer.PieceJob, error) {

	// Validate torrent metadata.
	if metadata.PieceLength <= 0 || metadata.TotalLength < 0 {
		return nil, fmt.Errorf("invalid torrent metadata")
	}

	// Calculate expected number of pieces.
	expectedPieces := (metadata.TotalLength + int64(metadata.PieceLength) - 1) /
		int64(metadata.PieceLength)

	if len(metadata.PieceHashes) != int(expectedPieces) {
		return nil, fmt.Errorf("piece hash count does not match expected piece count")
	}

	// No jobs are required for an empty file.
	if expectedPieces == 0 {
		return []peer.PieceJob{}, nil
	}

	// Ensure peers are available to download from.
	if trackerResponse == nil || len(trackerResponse.Peers) == 0 {
		return nil, fmt.Errorf("no peers available")
	}

	jobs := make([]peer.PieceJob, 0, expectedPieces)

	// Distribute pieces across available peers using round-robin.
	for i := 0; i < int(expectedPieces); i++ {
		currentPeer := trackerResponse.Peers[i%len(trackerResponse.Peers)]

		// Calculate actual piece length, accounting for the final piece.
		pieceLength := uint32(metadata.PieceLength)
		remaining := metadata.TotalLength - int64(i)*int64(metadata.PieceLength)

		if remaining < int64(pieceLength) {
			pieceLength = uint32(remaining)
		}

		// Construct peer address.
		peerAddress := net.JoinHostPort(
			currentPeer.IPAddress,
			strconv.Itoa(currentPeer.Port),
		)

		// Create piece download job.
		job := peer.PieceJob{
			PieceIndex:   uint32(i),
			Infohash:     infoHash,
			LocalPeerID:  localPeerID,
			PeerAddress:  peerAddress,
			PieceLength:  pieceLength,
			ExpectedHash: metadata.PieceHashes[i],
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func DownloadFromSwarm(trackerURL string, address string, peerID [20]byte, port int, infoHash [32]byte, transactionID int, metadata chunker.TorrentMetaData, outputPath string, workerCount int) error {
	infoHashHex := hex.EncodeToString(infoHash[:])

	//Announce client to tracker and get avaliable peers
	trackerResponse, err := Announce(trackerURL, string(peerID[:]), port, infoHashHex, transactionID, "started")
	if err != nil {
		return err
	}

	//Generate jobs
	jobs, err := CreatePieceJobs(trackerResponse, metadata, infoHash, peerID)
	if err != nil {
		return err
	}
	//Distrbute jobs across workers for concurrent piece downloads. And start our server for seeding
	err = DownloadFile(jobs, workerCount, outputPath, metadata.PieceLength, metadata.TotalLength)
	if err != nil {
		return err
	}

	err = StartSeeding(address, outputPath, uint32(metadata.PieceLength), trackerURL, string(peerID[:]), port, infoHashHex, transactionID)
	if err != nil {
		return err
	}
	return nil

}

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	ws "github.com/prdethxn/BitTorrent-Lyte/internal/websocket"

	"github.com/prdethxn/BitTorrent-Lyte/bitfield"
	"github.com/prdethxn/BitTorrent-Lyte/chunker"
	"github.com/prdethxn/BitTorrent-Lyte/client"
	"github.com/prdethxn/BitTorrent-Lyte/peer"
)

// Every client needs a unique 20-byte peer ID.
func generatePeerID() ([20]byte, error) {
	var peerID [20]byte

	_, err := rand.Read(peerID[:])
	if err != nil {
		return [20]byte{}, fmt.Errorf("failed to generate peer ID: %w", err)
	}

	return peerID, nil
}

// Start the WebSocket broadcaster and HTTP server.
func startWebSocketServer(hub *ws.Hub, wsPort int) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWs)

	addr := fmt.Sprintf("localhost:%d", wsPort)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to start WebSocket server: %w", err)
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	fmt.Printf("WebSocket server running at ws://%s/ws\n", addr)

	return server, nil
}

// Validate command-line arguments.
func validateArguments(
	mode, filePath, torrentPath, outputPath string,
	port, workers, pieceSize, wsPort int,
) error {

	if mode != "seed" && mode != "download" {
		return fmt.Errorf("mode must be seed or download")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	if wsPort < 1 || wsPort > 65535 {
		return fmt.Errorf("ws-port must be between 1 and 65535")
	}

	if workers < 1 {
		return fmt.Errorf("worker count must be positive")
	}

	if pieceSize < 1 {
		return fmt.Errorf("piece size must be positive")
	}

	if mode == "seed" && (filePath == "" || torrentPath == "") {
		return fmt.Errorf("seed mode requires -file and -torrent")
	}

	if mode == "download" && (torrentPath == "" || outputPath == "") {
		return fmt.Errorf("download mode requires -torrent and -output")
	}

	return nil
}

// Generate metadata, register seeder and start uploading.
func runSeed(
	filePath, torrentPath, trackerURL, address string,
	port, pieceSize int,
	hub *ws.Hub,
) error {

	hub.Broadcast(ws.Event{
		Type:    "seeding_started",
		Message: "Starting seeding",
	})

	// Generate torrent metadata.
	metadata, err := chunker.ChunkFile(filePath, pieceSize)
	if err != nil {
		return err
	}

	jsonMetadata, err := chunker.ToJSON(&metadata)
	if err != nil {
		return err
	}

	if err := chunker.SaveTorrent(jsonMetadata, torrentPath); err != nil {
		return err
	}

	fmt.Println("Torrent metadata generated successfully.")

	// Generate infohash.
	infoHash, err := chunker.GenerateInfoHash(metadata)
	if err != nil {
		return err
	}

	infoHashHex := hex.EncodeToString(infoHash[:])

	// Generate peer ID.
	peerIDBytes, err := generatePeerID()
	if err != nil {
		return err
	}

	peerID := string(peerIDBytes[:])

	// Initialise bitfield.
	numPieces := (metadata.TotalLength + int64(metadata.PieceLength) - 1) /
		int64(metadata.PieceLength)

	availablePieces := bitfield.NewBitfield(int(numPieces))

	// Seeder possesses every piece.
	for i := 0; i < int(numPieces); i++ {
		if err := bitfield.MarkAsDownloaded(availablePieces, i); err != nil {
			return err
		}
	}

	// Register seeder with tracker.
	_, err = client.Announce(
		trackerURL,
		peerID,
		port,
		infoHashHex,
		1,
		"started",
	)
	if err != nil {
		return err
	}

	// Start upload server.
	err = client.StartSeeding(
		address,
		filePath,
		uint32(metadata.PieceLength),
		trackerURL,
		peerID,
		port,
		infoHashHex,
		1,
		availablePieces,
	)
	if err != nil {
		hub.Broadcast(ws.Event{
			Type:    "error",
			Message: fmt.Sprintf("Failed to start seeding: %v", err),
		})

		return err
	}

	hub.Broadcast(ws.Event{
		Type:    "seeding_ready",
		Message: "Seeder is running and waiting for peer connections",
	})

	fmt.Println("Seeder running. Waiting for peer connections.")

	return nil
}

// Load torrent metadata and download pieces from the swarm.
func runDownload(
	torrentPath, outputPath, trackerURL, address string,
	port, workerCount int,
	hub *ws.Hub,
) error {

	hub.Broadcast(ws.Event{
		Type:    "download_started",
		Message: "Starting file download",
	})

	// Load torrent metadata.
	metadata, err := chunker.LoadTorrent(torrentPath)
	if err != nil {
		return err
	}

	// Generate infohash.
	infoHash, err := chunker.GenerateInfoHash(metadata)
	if err != nil {
		return err
	}

	// Generate peer ID.
	peerID, err := generatePeerID()
	if err != nil {
		return err
	}

	totalPieces := len(metadata.PieceHashes)

	if totalPieces == 0 {
		return fmt.Errorf("torrent contains no pieces")
	}

	// Track download progress safely across workers.
	var mu sync.Mutex

	completedPieces := 0
	successfulPieces := 0
	failedPieces := 0

	// Process individual piece results.
	onResult := func(result peer.PieceResult) {

		mu.Lock()
		defer mu.Unlock()

		completedPieces++

		if result.Err != nil {
			failedPieces++

			hub.Broadcast(ws.Event{
				Type: "piece_failed",
				Message: fmt.Sprintf(
					"Piece %d failed: %v",
					result.PieceIndex,
					result.Err,
				),
				Data: map[string]any{
					"piece_index": result.PieceIndex,
				},
			})

		} else {
			successfulPieces++

			hub.Broadcast(ws.Event{
				Type: "piece_downloaded",
				Message: fmt.Sprintf(
					"Piece %d downloaded successfully",
					result.PieceIndex,
				),
				Data: map[string]any{
					"piece_index": result.PieceIndex,
				},
			})
		}

		percentage := float64(completedPieces) /
			float64(totalPieces) * 100

		hub.Broadcast(ws.Event{
			Type:    "download_progress",
			Message: "Download progress updated",
			Data: map[string]any{
				"completed_pieces":  completedPieces,
				"successful_pieces": successfulPieces,
				"failed_pieces":     failedPieces,
				"total_pieces":      totalPieces,
				"percentage":        percentage,
			},
		})
	}

	// Download file from swarm.
	err = client.DownloadFromSwarm(
		trackerURL,
		address,
		peerID,
		port,
		infoHash,
		1,
		metadata,
		outputPath,
		workerCount,
		onResult,
	)

	if err != nil {
		hub.Broadcast(ws.Event{
			Type:    "error",
			Message: fmt.Sprintf("Download failed: %v", err),
		})

		return err
	}

	// Notify clients that downloading has completed.
	hub.Broadcast(ws.Event{
		Type:    "download_completed",
		Message: "File downloaded successfully",
	})

	fmt.Println("Download completed successfully.")

	hub.Broadcast(ws.Event{
		Type:    "seeding_started",
		Message: "Now seeding downloaded file",
	})

	fmt.Println("Now seeding downloaded file.")

	return nil
}

// Keep the client running while its upload server is active.
func waitForShutdown() {
	stop := make(chan os.Signal, 1)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	<-stop
}

// Gracefully shut down the WebSocket HTTP server.
func shutdownWebSocketServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("WebSocket shutdown error: %v", err)
	}
}

func main() {

	mode := flag.String("mode", "", "Client mode: seed or download")
	filePath := flag.String("file", "", "File to seed")
	torrentPath := flag.String("torrent", "", "Torrent metadata file")
	outputPath := flag.String("output", "", "Downloaded file destination")

	trackerURL := flag.String(
		"tracker",
		"http://localhost:8080/announce",
		"Tracker URL",
	)

	address := flag.String(
		"address",
		"0.0.0.0",
		"Local listening address",
	)

	port := flag.Int("port", 6881, "Peer listening port")
	workerCount := flag.Int("workers", 5, "Number of download workers")
	pieceSize := flag.Int("piece-size", 256*1024, "Piece size in bytes")
	wsPort := flag.Int("ws-port", 8081, "WebSocket server listening port")

	flag.Parse()

	// Validate arguments.
	if err := validateArguments(
		*mode,
		*filePath,
		*torrentPath,
		*outputPath,
		*port,
		*workerCount,
		*pieceSize,
		*wsPort,
	); err != nil {
		fmt.Println(err)
		return
	}

	// Initialise WebSocket Hub.
	hub := ws.NewHub()
	go hub.Run()

	// Notify connected clients that the BitTorrent client has started.
	hub.Broadcast(ws.Event{
		Type:    "client_started",
		Message: "BitTorrent client initialised successfully",
	})

	// Start WebSocket HTTP server.
	wsServer, err := startWebSocketServer(hub, *wsPort)
	if err != nil {
		log.Printf("WebSocket startup failed: %v", err)
		return
	}

	defer shutdownWebSocketServer(wsServer)

	// Construct TCP listening address.
	listenAddress := net.JoinHostPort(
		*address,
		strconv.Itoa(*port),
	)

	// Execute selected client mode.
	if *mode == "seed" {

		err = runSeed(
			*filePath,
			*torrentPath,
			*trackerURL,
			listenAddress,
			*port,
			*pieceSize,
			hub,
		)

	} else {

		err = runDownload(
			*torrentPath,
			*outputPath,
			*trackerURL,
			listenAddress,
			*port,
			*workerCount,
			hub,
		)
	}

	if err != nil {
		fmt.Println("Client error:", err)
		return
	}

	// Keep client running.
	waitForShutdown()
}

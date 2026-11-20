package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/prdethxn/BitTorrent-Lyte/bitfield"
	"github.com/prdethxn/BitTorrent-Lyte/chunker"
	"github.com/prdethxn/BitTorrent-Lyte/client"
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

func validateArguments(mode, filePath, torrentPath, outputPath string, port, workers, pieceSize int) error {
	if mode != "seed" && mode != "download" {
		return fmt.Errorf("mode must be seed or download")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
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
func runSeed(filePath, torrentPath, trackerURL, address string, port, pieceSize int) error {

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
	_, err = client.Announce(trackerURL, peerID, port, infoHashHex, 1, "started")
	if err != nil {
		return err
	}

	// Start upload server and announce completion.
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
		return err
	}

	fmt.Println("Seeder running. Waiting for peer connections.")

	return nil
}

// Load torrent metadata and download pieces from the swarm.
func runDownload(torrentPath, outputPath, trackerURL, address string, port, workerCount int) error {

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

	// Download file and start seeding.
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
	)

	if err != nil {
		return err
	}

	fmt.Println("Download completed successfully.")
	fmt.Println("Now seeding downloaded file.")

	return nil
}

// Keep the client running while its upload server is active.
func waitForShutdown() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}

func main() {
	mode := flag.String("mode", "", "Client mode: seed or download")
	filePath := flag.String("file", "", "File to seed")
	torrentPath := flag.String("torrent", "", "Torrent metadata file")
	outputPath := flag.String("output", "", "Downloaded file destination")
	trackerURL := flag.String("tracker", "http://localhost:8080/announce", "Tracker URL")
	address := flag.String("address", "0.0.0.0", "Local listening address")
	port := flag.Int("port", 6881, "Peer listening port")
	workerCount := flag.Int("workers", 5, "Number of download workers")
	pieceSize := flag.Int("piece-size", 256*1024, "Piece size in bytes")

	flag.Parse()

	if err := validateArguments(
		*mode, *filePath, *torrentPath, *outputPath,
		*port, *workerCount, *pieceSize,
	); err != nil {
		fmt.Println(err)
		return
	}

	// Construct TCP listening address.
	listenAddress := net.JoinHostPort(*address, strconv.Itoa(*port))

	var err error

	if *mode == "seed" {
		err = runSeed(
			*filePath,
			*torrentPath,
			*trackerURL,
			listenAddress,
			*port,
			*pieceSize,
		)
	} else {
		err = runDownload(
			*torrentPath,
			*outputPath,
			*trackerURL,
			listenAddress,
			*port,
			*workerCount,
		)
	}

	if err != nil {
		fmt.Println("Client error:", err)
		return
	}

	// Keep upload server alive.
	waitForShutdown()
}

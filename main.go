package main

import (
	"flag"
	"fmt"
)

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

	if mode == "seed" && filePath == "" {
		return fmt.Errorf("seed mode requires -file")
	}

	if mode == "download" && (torrentPath == "" || outputPath == "") {
		return fmt.Errorf("download mode requires -torrent and -output")
	}

	return nil
}

func main() {
	mode := flag.String("mode", "", "Client mode: seed or download")
	filePath := flag.String("file", "", "File to seed")
	torrentPath := flag.String("torrent", "", "Torrent metadata file")
	outputPath := flag.String("output", "", "Downloaded file destination")
	trackerURL := flag.String("tracker", "http://localhost:8080/announce", "Tracker URL")
	address := flag.String("address", "localhost", "Local listening address")
	port := flag.Int("port", 6881, "Peer listening port")
	workerCount := flag.Int("workers", 5, "Number of download workers")
	pieceSize := flag.Int("piece-size", 256*1024, "Piece size in bytes")

	flag.Parse()
	if err := validateArguments(*mode, *filePath, *torrentPath, *outputPath, *port, *workerCount, *pieceSize); err != nil {
		fmt.Println(err)
		return
	}
}

package chunker

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
)

// Turn MetaData to JSON
func ToJSON(m *TorrentMetaData) (string, error) {
	// Convert TorrentMetaData to JSON
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	//Turn JSON to readable string
	return string(jsonBytes), nil

}

// Write JSON to torrent file
func SaveTorrent(jsonString string, filename string) error {
	return os.WriteFile(filename, []byte(jsonString), 0644)

}

// Read torrent file
func LoadTorrent(filename string) (TorrentMetaData, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return TorrentMetaData{}, fmt.Errorf("failed to read file: %w", err)
	}
	//convert JSON to torrentMetaData
	metadata := TorrentMetaData{}
	err = json.Unmarshal(data, &metadata)

	if err != nil {
		return TorrentMetaData{}, fmt.Errorf("failed to decode torrent metadata: %w", err)
	}
	return metadata, nil
}

// Generate a consistent SHA-256 infohash from torrent metadata.
func GenerateInfoHash(metadata TorrentMetaData) ([32]byte, error) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return sha256.Sum256(data), nil
}

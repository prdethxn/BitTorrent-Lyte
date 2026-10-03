package chunker

import (
	"encoding/json"
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

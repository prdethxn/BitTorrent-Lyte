package chunker

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Metadata structure for torrent files
type TorrentMetaData struct {
	Filename    string   `json:"filename"`
	TotalLength int64    `json:"total_length"`
	PieceLength int      `json:"piece_length"` //Maximum size of each piece in bytes
	PieceHashes []string `json:"piece_hashes"`
}

//Divide file into chunks and return metatdata

func validateChunkSize(chunkSize int) bool {
	return chunkSize > 0
}

func chunkFile(filePath string, chunkSize int) (TorrentMetaData, error) {

	if !validateChunkSize(chunkSize) {
		return TorrentMetaData{}, errors.New("Chunk size must be > 0")
	}
	//open file
	file, err := os.Open(filePath)
	if err != nil {
		return TorrentMetaData{}, fmt.Errorf("Failed to open file: %v", err)
	}
	defer file.Close()

	//read file in chunks. Use fixed-size byte buffer to not load entire file into RAM
	//chunkSize is the MAX size of chunk, so this is a buffer of that size
	metaData := TorrentMetaData{Filename: filepath.Base(filePath), PieceLength: chunkSize}
	buffer := make([]byte, chunkSize)
	totalBytes := int64(0)
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return TorrentMetaData{}, fmt.Errorf("Failed to read file: %v", err)
		}
		totalBytes += int64(bytesRead)

		if bytesRead > 0 {
			chunk := buffer[:bytesRead]
			//Compute chunk hash
			hash := sha256.Sum256(chunk)
			//Append hash to metadata, converting it to string
			metaData.PieceHashes = append(metaData.PieceHashes, fmt.Sprintf("%x", hash))

		}
		//Exit loop when done reading file.
		if err == io.EOF {
			metaData.TotalLength = totalBytes
			break
		}
	}

	return metaData, nil

}

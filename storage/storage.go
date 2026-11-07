package storage

import (
	"fmt"
	"os"
)

// Reconstruct the file
func AssembleFile(outputPath string, pieces map[uint32][]byte, pieceLength int, totalLength int64) error {
	if pieceLength <= 0 || totalLength < 0 {
		return fmt.Errorf("invalid piece length or total length of file")
	}

	//determine expect num of pieces
	expectedPieces := (totalLength + int64(pieceLength) - 1) / int64(pieceLength)
	if len(pieces) != int(expectedPieces) {
		return fmt.Errorf("incorrect number of pieces have been recieved")
	}

	//validate piece indexes
	for i := uint32(0); i < uint32(expectedPieces); i++ {
		_, exists := pieces[i]
		if !exists {
			return fmt.Errorf("expected piece index missing")
		}

		//validate piece length
		expectedSize := int64(pieceLength)

		if i == uint32(expectedPieces-1) {
			expectedSize = totalLength - (int64(i) * int64(pieceLength))
		}

		if int64(len(pieces[i])) != expectedSize {
			return fmt.Errorf("invalid piece length at index %d", i)
		}
	}

	//create file
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	//write pieces to file
	for i := 0; i < int(expectedPieces); i++ {
		offset := i * pieceLength
		n, err := file.WriteAt(pieces[uint32(i)], int64(offset))
		if err != nil {
			return err
		}

		//check for short wrties
		if n != len(pieces[uint32(i)]) {
			return fmt.Errorf("failure: shortwrite occured")
		}
	}
	return nil
}

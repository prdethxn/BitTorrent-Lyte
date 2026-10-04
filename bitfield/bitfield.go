package bitfield

import "fmt"

// Conveys pieces which the peer has downloaded
type Bitfield struct {
	field       []byte
	numOfPieces int
}

// Check if index is valid for bitfield
func validIndex(b *Bitfield, index int) bool {
	return 0 <= index && index < b.numOfPieces
}

// Constructor function.
func newBitfield(numOfPieces int) *Bitfield {
	return &Bitfield{
		numOfPieces: numOfPieces,
		field:       make([]byte, (numOfPieces+7)/8),
	}

}

func markAsDownloaded(b *Bitfield, index int) error {
	if !validIndex(b, index) {
		return fmt.Errorf("Index out of bounds")
	}
	//Allow index to access correct bit so the right piece is marked
	byteIndex := index / 8
	bitIndex := index % 8
	b.field[byteIndex] |= (1 << (7 - bitIndex))
	return nil
}

func isDownloaded(b *Bitfield, index int) (bool, error) {
	if !validIndex(b, index) {
		return false, fmt.Errorf("Index out of bounds")
	}
	byteIndex := index / 8
	bitIndex := index % 8
	return b.field[byteIndex]&(1<<(7-bitIndex)) != 0, nil
}

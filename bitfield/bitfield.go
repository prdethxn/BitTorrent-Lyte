package bitfield

import (
	"fmt"
	"sync"
)

// Conveys pieces which the peer has downloaded
type Bitfield struct {
	field       []byte
	numOfPieces int
	mu          sync.RWMutex
}

// Check if index is valid for bitfield
func validIndex(b *Bitfield, index int) bool {
	return 0 <= index && index < b.numOfPieces
}

// Constructor function.
func NewBitfield(numOfPieces int) *Bitfield {
	if numOfPieces < 0 {
		return nil
	}
	return &Bitfield{
		numOfPieces: numOfPieces,
		field:       make([]byte, (numOfPieces+7)/8),
	}

}

func MarkAsDownloaded(b *Bitfield, index int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !validIndex(b, index) {
		return fmt.Errorf("Index out of bounds")
	}
	//Allow index to access correct bit so the right piece is marked
	byteIndex := index / 8
	bitIndex := index % 8
	b.field[byteIndex] |= (1 << (7 - bitIndex))
	return nil
}

func IsDownloaded(b *Bitfield, index int) (bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if !validIndex(b, index) {
		return false, fmt.Errorf("Index out of bounds")
	}
	byteIndex := index / 8
	bitIndex := index % 8
	return b.field[byteIndex]&(1<<(7-bitIndex)) != 0, nil
}

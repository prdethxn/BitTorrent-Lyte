package integrity

import (
	"crypto/sha256"
	"encoding/hex"
)

// Consume incoming chunk and verify integrity through SHA-256 hashing
func VerifyIntegrity(data []byte, expectedHash string) bool {
	currentHash := sha256.Sum256(data)
	return hex.EncodeToString(currentHash[:]) == expectedHash

}

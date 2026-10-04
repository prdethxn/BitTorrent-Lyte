package integrity

import (
	"testing"
)

func TestVerifyIntegrity(t *testing.T) {
	//test with valid data and hash

	data := []byte("Hello")
	expectedHash := []string{"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969"}
	if !verifyIntegrity(0, data, expectedHash) {
		t.Errorf("Integrity check failed for valid data and hash")
	}

	//test with invalid hash and data
	invalidData := []byte("Jello")
	invalidHash := []string{"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969"}
	if verifyIntegrity(0, invalidData, invalidHash) {
		t.Errorf("Integrity check passed for invalid data and hash combination")
	}

}

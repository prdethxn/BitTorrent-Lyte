package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAssembleFileReconstructsPiecesInOrder(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "reconstructed.bin")
	pieces := map[uint32][]byte{
		2: []byte("ij"),
		0: []byte("abcd"),
		1: []byte("efgh"),
	}

	if err := AssembleFile(outputPath, pieces, 4, 10); err != nil {
		t.Fatalf("AssembleFile returned error: %v", err)
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read reconstructed file: %v", err)
	}
	want := []byte("abcdefghij")
	if !bytes.Equal(got, want) {
		t.Fatalf("reconstructed contents = %q, want %q", got, want)
	}
}

func TestAssembleFileCreatesEmptyFile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "empty.bin")

	if err := AssembleFile(outputPath, map[uint32][]byte{}, 4, 0); err != nil {
		t.Fatalf("AssembleFile rejected empty file: %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat empty output: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("empty output size = %d, want 0", info.Size())
	}
}

func TestAssembleFileRejectsInvalidLengths(t *testing.T) {
	tests := []struct {
		name        string
		pieceLength int
		totalLength int64
	}{
		{name: "zero piece length", pieceLength: 0, totalLength: 1},
		{name: "negative piece length", pieceLength: -1, totalLength: 1},
		{name: "negative total length", pieceLength: 4, totalLength: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := AssembleFile(filepath.Join(t.TempDir(), "out.bin"),
				map[uint32][]byte{}, test.pieceLength, test.totalLength)
			if err == nil {
				t.Fatal("AssembleFile succeeded with invalid lengths")
			}
		})
	}
}

func TestAssembleFileRejectsMissingPiece(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "out.bin")
	pieces := map[uint32][]byte{
		0: []byte("abcd"),
		2: []byte("ij"),
	}

	if err := AssembleFile(outputPath, pieces, 4, 10); err == nil {
		t.Fatal("AssembleFile succeeded with a missing piece")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output should not be created for invalid pieces; stat error = %v", err)
	}
}

func TestAssembleFileRejectsIncorrectPieceCount(t *testing.T) {
	pieces := map[uint32][]byte{
		0: []byte("abcd"),
		1: []byte("efgh"),
		2: []byte("ij"),
		3: []byte("kl"),
	}

	err := AssembleFile(filepath.Join(t.TempDir(), "out.bin"), pieces, 4, 10)
	if err == nil {
		t.Fatal("AssembleFile accepted more pieces than expected")
	}
}

func TestAssembleFileRejectsIncorrectPieceLength(t *testing.T) {
	tests := []struct {
		name   string
		pieces map[uint32][]byte
	}{
		{
			name: "short non-final piece",
			pieces: map[uint32][]byte{
				0: []byte("abc"),
				1: []byte("efgh"),
				2: []byte("ij"),
			},
		},
		{
			name: "incorrect final piece length",
			pieces: map[uint32][]byte{
				0: []byte("abcd"),
				1: []byte("efgh"),
				2: []byte("i"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := AssembleFile(filepath.Join(t.TempDir(), "out.bin"),
				test.pieces, 4, 10)
			if err == nil {
				t.Fatal("AssembleFile accepted a piece with an incorrect length")
			}
		})
	}
}

func TestAssembleFileReportsOutputCreationFailure(t *testing.T) {
	// A directory cannot be opened as the output file.
	outputPath := t.TempDir()
	err := AssembleFile(outputPath, map[uint32][]byte{}, 4, 0)
	if err == nil {
		t.Fatal("AssembleFile succeeded when output path was a directory")
	}
}

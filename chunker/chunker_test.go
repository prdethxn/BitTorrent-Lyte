package chunker

import (
	"reflect"
	"testing"
)

func TestValidateChunkSize(t *testing.T) {
	got := validateChunkSize(0)
	want := false
	if got != want {
		t.Errorf("validateChunkSize(0) = %v; want %v", got, want)
	}
}

func TestChunkFile(t *testing.T) {
	got, err := chunkFile("test_files/testinput.txt", 5)
	want := TorrentMetaData{
		Filename:    "testinput.txt",
		TotalLength: 12,
		PieceLength: 5,
		PieceHashes: []string{
			"185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969",
			"d09e01c15401fa703eaa271a5807bee8e8db66eb1df6a46cc1d9ca5d6d9ce926",
			"2615739d41db311a978c3232f5189cb5172f13ddc862712d4d750ff463b9cb8a",
		},
	}
	wantErr := error(nil)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("chunkFile(\"test_files/testinput.txt\", 5) = %v; want %v", got, want)
	}
	if err != wantErr {
		t.Errorf("chunkFile(\"test_files/testinput.txt\", 5) error = %v; want %v", err, wantErr)
	}

}

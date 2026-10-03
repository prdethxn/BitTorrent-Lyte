package chunker

import (
	"os"
	"testing"
)

func TestToJSON(t *testing.T) {
	testMetaData, testErr := chunkFile("test_files/testinput.txt", 5)
	if testErr != nil {
		t.Fatalf("chunkFile failed: %v", testErr)
	}

	got, err := ToJSON(&testMetaData)
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}
	want := `{"filename":"testinput.txt","total_length":12,"piece_length":5,"piece_hashes":["185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969","d09e01c15401fa703eaa271a5807bee8e8db66eb1df6a46cc1d9ca5d6d9ce926","2615739d41db311a978c3232f5189cb5172f13ddc862712d4d750ff463b9cb8a"]}`
	if got != want {
		t.Errorf("ToJson(&testMetaData) = %v; want %v", got, want)
	}
}

func TestSaveTorrent(t *testing.T) {
	want := `{"filename":"testinput.txt","total_length":12,"piece_length":5,"piece_hashes":["185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969","d09e01c15401fa703eaa271a5807bee8e8db66eb1df6a46cc1d9ca5d6d9ce926","2615739d41db311a978c3232f5189cb5172f13ddc862712d4d750ff463b9cb8a"]}`
	err := SaveTorrent(`{"filename":"testinput.txt","total_length":12,"piece_length":5,"piece_hashes":["185f8db32271fe25f561a6fc938b2e264306ec304eda518007d1764826381969","d09e01c15401fa703eaa271a5807bee8e8db66eb1df6a46cc1d9ca5d6d9ce926","2615739d41db311a978c3232f5189cb5172f13ddc862712d4d750ff463b9cb8a"]}`, "test_files/testoutput.torrent")
	if err != nil {
		t.Fatalf("SaveTorrent failed: %v", err)
	}

	//Check if file exists and contents match

	if _, err := os.Stat("test_files/testoutput.torrent"); os.IsNotExist(err) {
		t.Fatalf("File test_files/testoutput.torrent does not exist after SaveTorrent")
	}

	data, err := os.ReadFile("test_files/testoutput.torrent")
	if err != nil {
		t.Fatalf("Failed to read torrent file: %v", err)
	}
	if string(data) != want {
		t.Errorf("Contents of test file and torrent don't match. Got: %s", string(data))
	}
}
